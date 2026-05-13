// Package router provides embedded static file system for the admin panel.
package router

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var embedFS embed.FS

// StaticFiles 返回嵌入的前端静态文件系统
//
// 使用 Go embed 将管理后台的 HTML/CSS/JS 文件编译进二进制。
// 通过 Gin 的 StaticFS 挂载到 /admin 路径下。
//
// 返回:
//   - http.FileSystem: 可用于 gin.StaticFS 的文件系统
func StaticFiles() http.FileSystem {
	sub, err := fs.Sub(embedFS, "static")
	if err != nil {
		// 理论上不应发生，因为 embed 在编译时验证
		return http.Dir("static")
	}
	return http.FS(sub)
}

// AdminRedirect 处理根路径重定向到管理后台
func AdminRedirect(c *gin.Context) {
	c.Redirect(302, "/admin/")
}
