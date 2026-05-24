package edge

import (
	"testing"
)

func TestCompileValidScript(t *testing.T) {
	src := `
function transform(req, resp)
    resp.body = string.gsub(resp.body, "old", "new")
    return resp
end`
	proto, err := compileLuaScript(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if proto == nil {
		t.Fatal("proto should not be nil")
	}
}

func TestCompileInvalidScript(t *testing.T) {
	src := `this is not valid lua @@`
	_, err := compileLuaScript(src)
	if err == nil {
		t.Fatal("expected error for invalid syntax")
	}
}

func TestCompileMissingTransform(t *testing.T) {
	src := `-- no transform function defined`
	_, err := compileLuaScript(src)
	if err == nil {
		t.Fatal("expected error for missing transform function")
	}
}

func TestExecuteTransform(t *testing.T) {
	src := `
function transform(req, resp)
    resp.body = string.gsub(resp.body, "old", "new")
    resp.headers["x-custom"] = "test"
    return resp
end`
	proto, err := compileLuaScript(src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	pool := newLStatePool()
	se := &scriptEngine{proto: proto}

	req := &scriptReq{Method: "GET", Path: "/"}
	resp := &scriptResp{
		StatusCode: 200,
		Headers:    map[string]string{"content-type": "text/html"},
		Body:       "hello old world",
	}

	applyLuaScript(se, pool, req, resp, defaultScriptTimeout)

	if resp.Body != "hello new world" {
		t.Errorf("body = %q, want %q", resp.Body, "hello new world")
	}
	if resp.Headers["x-custom"] != "test" {
		t.Errorf("x-custom = %q, want %q", resp.Headers["x-custom"], "test")
	}
}

func TestEmptyScript(t *testing.T) {
	pool := newLStatePool()
	req := &scriptReq{Method: "GET", Path: "/"}
	resp := &scriptResp{StatusCode: 200, Body: "hello"}

	// nil engine
	applyLuaScript(nil, pool, req, resp, defaultScriptTimeout)
	if resp.Body != "hello" {
		t.Errorf("body changed: %q", resp.Body)
	}

	// engine with nil proto
	applyLuaScript(&scriptEngine{proto: nil}, pool, req, resp, defaultScriptTimeout)
	if resp.Body != "hello" {
		t.Errorf("body changed: %q", resp.Body)
	}
}

func TestGzipRoundTrip(t *testing.T) {
	src := `
function transform(req, resp)
    resp.body = string.gsub(resp.body, "old", "new")
    return resp
end`
	proto, _ := compileLuaScript(src)
	pool := newLStatePool()
	se := &scriptEngine{proto: proto}

	original := "hello old world"
	compressed, _ := gzipData([]byte(original))

	req := &scriptReq{Method: "GET"}
	resp := &scriptResp{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Encoding": "gzip"},
		Body:       string(compressed),
	}

	applyLuaScript(se, pool, req, resp, defaultScriptTimeout)

	// Decompress result for verification
	result, err := gunzip([]byte(resp.Body))
	if err != nil {
		t.Fatalf("gunzip error: %v", err)
	}
	if string(result) != "hello new world" {
		t.Errorf("body = %q, want %q", string(result), "hello new world")
	}
}

func TestDeflateRoundTrip(t *testing.T) {
	src := `
function transform(req, resp)
    resp.body = string.gsub(resp.body, "old", "new")
    return resp
end`
	proto, _ := compileLuaScript(src)
	pool := newLStatePool()
	se := &scriptEngine{proto: proto}

	original := "hello old world"
	compressed, _ := inflateData([]byte(original))

	req := &scriptReq{Method: "GET"}
	resp := &scriptResp{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Encoding": "deflate"},
		Body:       string(compressed),
	}

	applyLuaScript(se, pool, req, resp, defaultScriptTimeout)

	// Decompress result for verification
	result, err := deflateData([]byte(resp.Body))
	if err != nil {
		t.Fatalf("deflate error: %v", err)
	}
	if string(result) != "hello new world" {
		t.Errorf("body = %q, want %q", string(result), "hello new world")
	}
}
