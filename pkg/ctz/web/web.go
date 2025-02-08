package web

import (
	"ceph-to-zfs/pkg/ctz/logging"
	"ceph-to-zfs/pkg/ctz/status"
	"ceph-to-zfs/pkg/ctz/task"
	"ceph-to-zfs/pkg/ctz/util"
	"fmt"
	"net"
	"net/http"
	"os"
)
import "github.com/gin-gonic/gin"

func StartWebInterface(topLevel task.PreparableTask, port int) error {
	log := logging.NewRootLogger("Web")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: func(params gin.LogFormatterParams) string {
			if params.ErrorMessage == "" {
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
	r.GET("/api/alltasks", func(c *gin.Context) {
		c.JSON(http.StatusOK, ToTaskView(topLevel))
	})
	r.GET("/api/startall", func(c *gin.Context) {
		go topLevel.Run()
		c.JSON(http.StatusOK, gin.H{"Status": "Started"})
	})
	r.GET("/api/prepall", func(c *gin.Context) {
		go topLevel.Prepare()
		c.JSON(http.StatusOK, gin.H{"Status": "Started"})
	})
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

type TaskView struct {
	Id       string     `json:"id"`
	Label    string     `json:"label"`
	Status   StatusView `json:"status"`
	Children []TaskView `json:"children"`
}

func ToTaskView(t task.Task) TaskView {
	s := t.StatusLog().Status()
	st := s.Type()
	return TaskView{
		Id:    t.Id(),
		Label: t.Label(),
		Status: StatusView{
			Type:       st.Label(),
			Message:    s.Msg(),
			IsBad:      st.IsBad(),
			IsTerminal: st.IsTerminal(),
		},
		Children: util.Map(t.Children(), ToTaskView),
	}
}

type StatusView struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	IsBad      bool   `json:"isBad"`
	IsTerminal bool   `json:"isTerminal"`
}
