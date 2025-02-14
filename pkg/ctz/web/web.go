package web

import (
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"net"
	"os"
	"strings"
)
import "github.com/gin-gonic/gin"

func StartWebInterface(topLevel task.PreparableTask, port int) error {
	log := logging.NewRootLogger("Web")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Skip: func(c *gin.Context) bool {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api/alltasks") || strings.HasPrefix(path, "/api/taskdetails") {
				return true
			} else {
				return false
			}
		},
		SkipPaths: []string{"/api/alltasks", "/api/taskdetails"},
		Formatter: func(params gin.LogFormatterParams) string {
			if params.ErrorMessage == "" {
				// Don't log normal successful requests
				// TODO this doesn't work here - this can only reformat, not skip, and the
				// "Skip" field also doesn't seem to allow access to these fields.
				//if params.StatusCode != http.StatusOK {
				//	return fmt.Sprintf("(%v) %v %v %#v", params.ClientIP, params.StatusCode, params.Method, params.Path)
				//}
				return fmt.Sprintf("(%v) %v %v %#v", params.ClientIP, params.StatusCode, params.Method, params.Path)
			} else {
				return fmt.Sprintf("(%v) %v %v %#v\n%v", params.ClientIP, params.StatusCode, params.Method, params.Path, params.ErrorMessage)
			}
		},
		Output: log.AsWriter(),
	}))
	r.Use(gin.Recovery())
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	})
	wa := NewWebApi(topLevel)
	wa.InstallRoutes(r.Group("/api"))
	InstallFrontend(r)
	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Log("Failed to listen: %v", err)
		return err
	}
	go func() {
		log.SetStatus(status.MakeStatus(status.Active, fmt.Sprintf("Running web server on port %d", port)))
		err = r.RunListener(listen)
		if err != nil {
			log.SetStatusByError(err)
			os.Exit(3)
		}
	}()
	return nil
}
