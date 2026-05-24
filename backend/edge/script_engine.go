package edge

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

const (
	defaultScriptTimeout = 500 * time.Millisecond
)

// scriptEngine holds the compiled Lua function for a single domain.
type scriptEngine struct {
	proto *lua.FunctionProto // pre-compiled transform function
}

// scriptReq is the request-side data exposed to Lua.
type scriptReq struct {
	Method  string
	Path    string
	Query   string
	Headers map[string]string
}

// scriptResp is the response-side data read/written by Lua.
type scriptResp struct {
	StatusCode int
	Headers    map[string]string
	Body       string
}

// compileLuaScript compiles a Lua script and validates it has a transform function.
// Returns (nil, error) on any failure.
func compileLuaScript(source string) (*lua.FunctionProto, error) {
	L := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})
	defer L.Close()

	// Load only safe libraries
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	} {
		if err := L.CallByParam(lua.P{
			Fn:      L.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			return nil, fmt.Errorf("open lib %s: %w", pair.n, err)
		}
	}

	// Compile the script
	chunk, err := parse.Parse(strings.NewReader(source), "<edge_script>")
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	proto, err := lua.Compile(chunk, "<edge_script>")
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	// Run the chunk to register transform function
	lfunc := L.NewFunctionFromProto(proto)
	L.Push(lfunc)
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		return nil, fmt.Errorf("exec chunk: %w", err)
	}

	// Validate transform function exists
	transformFn := L.GetGlobal("transform")
	if transformFn.Type() != lua.LTFunction {
		return nil, fmt.Errorf("script must define a 'transform(req, resp)' function")
	}

	return proto, nil
}

// lStatePool is a sync.Pool of *lua.LState.
// Each LState is pre-configured with the safe library set.
type lStatePool struct {
	pool sync.Pool
}

func newLStatePool() *lStatePool {
	return &lStatePool{
		pool: sync.Pool{
			New: func() interface{} {
				L := lua.NewState(lua.Options{
					SkipOpenLibs: true,
				})
				// Load safe libraries
				for _, pair := range []struct {
					n string
					f lua.LGFunction
				}{
					{lua.BaseLibName, lua.OpenBase},
					{lua.TabLibName, lua.OpenTable},
					{lua.StringLibName, lua.OpenString},
					{lua.MathLibName, lua.OpenMath},
				} {
					_ = L.CallByParam(lua.P{
						Fn:      L.NewFunction(pair.f),
						NRet:    0,
						Protect: true,
					}, lua.LString(pair.n))
				}
				return L
			},
		},
	}
}

func (p *lStatePool) Get() *lua.LState {
	return p.pool.Get().(*lua.LState)
}

func (p *lStatePool) Put(L *lua.LState) {
	L.SetContext(context.TODO()) // clear context for reuse
	p.pool.Put(L)
}

func gunzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func gzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func deflateData(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func inflateData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// applyLuaScript executes the domain's Lua script against the response.
// It modifies the response in-place (body and headers).
// On any error, it logs and returns without modification (fail-open).
func applyLuaScript(se *scriptEngine, pool *lStatePool, req *scriptReq, resp *scriptResp, timeout time.Duration) {
	if se == nil || se.proto == nil {
		return
	}

	if timeout <= 0 {
		timeout = defaultScriptTimeout
	}

	// Handle Content-Encoding: decompress before Lua, recompress after
	ce := resp.Headers["Content-Encoding"]
	needsRecompress := false
	var originalBody []byte

	switch ce {
	case "gzip":
		decoded, err := gunzip([]byte(resp.Body))
		if err != nil {
			log.Printf("[edge] lua: gunzip error: %v", err)
			return
		}
		originalBody = []byte(resp.Body)
		resp.Body = string(decoded)
		needsRecompress = true
	case "deflate":
		decoded, err := deflateData([]byte(resp.Body))
		if err != nil {
			log.Printf("[edge] lua: deflate error: %v", err)
			return
		}
		originalBody = []byte(resp.Body)
		resp.Body = string(decoded)
		needsRecompress = true
	case "br":
		log.Printf("[edge] lua: brotli not supported, skipping script")
		return
	}

	L := pool.Get()
	defer pool.Put(L)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	L.SetContext(ctx)

	// Load the pre-compiled transform function
	lfunc := L.NewFunctionFromProto(se.proto)
	L.Push(lfunc)

	// Execute the chunk to make transform() available
	if err := L.PCall(0, lua.MultRet, nil); err != nil {
		log.Printf("[edge] lua: load transform: %v", err)
		if needsRecompress {
			resp.Body = string(originalBody)
		}
		return
	}

	// Build req table
	reqTable := L.NewTable()
	L.SetField(reqTable, "method", lua.LString(req.Method))
	L.SetField(reqTable, "path", lua.LString(req.Path))
	L.SetField(reqTable, "query", lua.LString(req.Query))
	reqHeadersTable := L.NewTable()
	for k, v := range req.Headers {
		L.SetField(reqHeadersTable, k, lua.LString(v))
	}
	L.SetField(reqTable, "headers", reqHeadersTable)

	// Build resp table (status_code, headers, body)
	respTable := L.NewTable()
	L.SetField(respTable, "status_code", lua.LNumber(resp.StatusCode))
	respHeadersTable := L.NewTable()
	for k, v := range resp.Headers {
		L.SetField(respHeadersTable, k, lua.LString(v))
	}
	L.SetField(respTable, "headers", respHeadersTable)
	L.SetField(respTable, "body", lua.LString(resp.Body))

	// Call transform(req, resp)
	tf := L.GetGlobal("transform")
	if err := L.CallByParam(lua.P{
		Fn:      tf,
		NRet:    1,
		Protect: true,
	}, reqTable, respTable); err != nil {
		log.Printf("[edge] lua: transform error: %v", err)
		if needsRecompress {
			resp.Body = string(originalBody)
		}
		return
	}

	// Lua's return value should be the modified resp table — we already have it via table mutation.
	// But to be safe, get from stack.
	ret := L.Get(-1)
	L.Pop(1)

	retTable, ok := ret.(*lua.LTable)
	if !ok {
		log.Printf("[edge] lua: transform must return a table")
		if needsRecompress {
			resp.Body = string(originalBody)
		}
		return
	}

	// Read back status_code
	if sc := retTable.RawGetString("status_code"); sc.Type() == lua.LTNumber {
		resp.StatusCode = int(lua.LVAsNumber(sc))
	}

	// Read back headers
	if ht := retTable.RawGetString("headers"); ht.Type() == lua.LTTable {
		newHeaders := make(map[string]string)
		ht.(*lua.LTable).ForEach(func(k, v lua.LValue) {
			if k.Type() == lua.LTString && v.Type() == lua.LTString {
				newHeaders[string(k.(lua.LString))] = string(v.(lua.LString))
			}
		})
		resp.Headers = newHeaders
	}

	// Read back body
	if b := retTable.RawGetString("body"); b.Type() == lua.LTString {
		resp.Body = string(b.(lua.LString))
	}

	// Recompress if needed
	if needsRecompress {
		switch ce {
		case "gzip":
			compressed, err := gzipData([]byte(resp.Body))
			if err != nil {
				log.Printf("[edge] lua: gzip error: %v", err)
				resp.Body = string(originalBody)
				return
			}
			resp.Body = string(compressed)
		case "deflate":
			compressed, err := inflateData([]byte(resp.Body))
			if err != nil {
				log.Printf("[edge] lua: deflate error: %v", err)
				resp.Body = string(originalBody)
				return
			}
			resp.Body = string(compressed)
		}
	}

	// Update Content-Length: remove and let Go auto-set
	// (resp.Headers uses lowercase keys from transformResponse)
	delete(resp.Headers, "content-length")
}

func NewScriptEngine(source string) *scriptEngine {
	if source == "" {
		return nil
	}
	proto, err := compileLuaScript(source)
	if err != nil {
		log.Printf("[edge] lua: compile error (script disabled): %v", err)
		return &scriptEngine{proto: nil} // non-nil but disabled
	}
	return &scriptEngine{proto: proto}
}
