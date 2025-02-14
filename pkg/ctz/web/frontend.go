package web

import (
	"embed"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
)

var (
	//go:embed webroot
	webroot embed.FS
)

func InstallFrontend(r *gin.Engine) {
	r.Use(static.Serve("", static.EmbedFolder(webroot, "webroot")))
}
