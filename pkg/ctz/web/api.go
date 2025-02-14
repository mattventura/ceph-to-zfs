package web

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/logging"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/task"
	"github.com/mattventura/ceph-to-zfs/pkg/ctz/util"
	"net/http"
	"strings"
	"time"
)

type Api struct {
	// t is the top-level task
	t task.PreparableTask
}

func NewWebApi(t task.PreparableTask) *Api {
	return &Api{t: t}
}

func (w *Api) InstallRoutes(r *gin.RouterGroup) {
	r.GET("/alltasks", w.AllTasks)
	r.GET("/startall", w.StartAll)
	r.GET("/prepall", w.PrepareAll)
	r.GET("/taskdetails/*task", w.TaskDetails)
}

func (w *Api) AllTasks(c *gin.Context) {
	c.JSON(http.StatusOK, ToTasksResponse(w.t))
}

func (w *Api) StartAll(c *gin.Context) {
	go w.t.Run()
	c.JSON(http.StatusOK, gin.H{"Status": "Started"})
}

func (w *Api) PrepareAll(c *gin.Context) {
	go w.t.Prepare()
	c.JSON(http.StatusOK, gin.H{"Status": "Started"})
}

func (w *Api) TaskDetails(c *gin.Context) {
	// TODO: root task
	taskPath := c.Param("task")
	parts := strings.Split(taskPath, "/")
	l := w.t.StatusLog()
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
