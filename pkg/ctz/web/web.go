package web

import (
	"fmt"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/status"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)
import "github.com/gin-gonic/gin"

func StartWebInterface(topLevel task.PreparableTask, port int) error {
	log := logging.NewRootLogger("Web")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/api/alltasks"},
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
	r.GET("/api/alltasks", func(c *gin.Context) {
		c.JSON(http.StatusOK, ToTasksResponse(topLevel))
	})
	r.GET("/api/startall", func(c *gin.Context) {
		go topLevel.Run()
		c.JSON(http.StatusOK, gin.H{"Status": "Started"})
	})
	r.GET("/api/prepall", func(c *gin.Context) {
		go topLevel.Prepare()
		c.JSON(http.StatusOK, gin.H{"Status": "Started"})
	})
	r.GET("/api/taskdetails/*task", func(c *gin.Context) {
		// TODO: root task
		taskPath := c.Param("task")
		parts := strings.Split(taskPath, "/")
		l := topLevel.StatusLog()
		var found bool
		for _, part := range parts {
			if part != "" {
				l, found = l.Children()[logging.LoggerKey(part)]
				if !found {
					c.JSON(http.StatusNotFound, gin.H{"Error": fmt.Sprintf("task %s not found", part)})
					return
				}
			}
		}
		c.JSON(http.StatusOK, ToTaskDetailResponse(l))

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

type TasksResponse struct {
	ServerInfo ServerInfo `json:"serverInfo"`
	Task       TaskView   `json:"task"`
}

type TaskDetailResponse struct {
	DetailData map[string]any `json:"detailData"`
}

type ServerInfo struct {
	UnixTime float64 `json:"unixTime"`
}

type TaskView struct {
	Id        string         `json:"id"`
	Label     string         `json:"label"`
	Status    StatusView     `json:"status"`
	ExtraData map[string]any `json:"extraData"`
	Children  []TaskView     `json:"children"`
}

func MakeServerInfo() ServerInfo {
	return ServerInfo{
		UnixTime: float64(time.Now().UnixMilli()) / 1000.0,
	}
}

func ToTasksResponse(t task.Task) TasksResponse {
	return TasksResponse{
		ServerInfo: MakeServerInfo(),
		Task:       ToTaskView(t),
	}
}

func ToTaskDetailResponse(l *logging.JobStatusLogger) TaskDetailResponse {
	return TaskDetailResponse{
		DetailData: l.GetDetailData(),
	}
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
			IsActive:   st.IsActive(),
		},
		ExtraData: t.StatusLog().GetExtraData(),
		Children:  util.Map(t.Children(), ToTaskView),
	}
}

type StatusView struct {
	Type       string `json:"type"`
	Message    string `json:"message"`
	IsBad      bool   `json:"isBad"`
	IsTerminal bool   `json:"isTerminal"`
	IsActive   bool   `json:"isActive"`
}
