package manager

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed static/*
var embedFS embed.FS

func StaticFiles() http.FileSystem {
	sub, err := fs.Sub(embedFS, "static")
	if err != nil {
		return http.Dir("static")
	}
	return http.FS(sub)
}

func AdminRedirect(c *gin.Context) {
	c.Redirect(302, "/admin/")
}
