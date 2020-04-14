package handler

import (
	"context"
	"net/http"

	"code.cloudfoundry.org/bbs/models"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager"
	"github.com/julienschmidt/httprouter"
)

//counterfeiter:generate . AppBifrost
type AppBifrost interface {
	Transfer(ctx context.Context, request cf.DesireLRPRequest) error
	List(ctx context.Context) ([]*models.DesiredLRPSchedulingInfo, error)
	Update(ctx context.Context, update cf.UpdateDesiredLRPRequest) error
	Stop(ctx context.Context, identifier opi.LRPIdentifier) error
	StopInstance(ctx context.Context, identifier opi.LRPIdentifier, index uint) error
	GetApp(ctx context.Context, identifier opi.LRPIdentifier) (*models.DesiredLRP, error)
	GetInstances(ctx context.Context, identifier opi.LRPIdentifier) ([]*cf.Instance, error)
}

//counterfeiter:generate . TaskBifrost
type TaskBifrost interface {
	TransferTask(ctx context.Context, taskGUID string, request cf.TaskRequest) error
}

//counterfeiter:generate . StagingBifrost
type StagingBifrost interface {
	TransferStaging(ctx context.Context, stagingGUID string, request cf.StagingRequest) error
	CompleteStaging(*models.TaskCallbackResponse) error
}

func New(bifrost AppBifrost,
	buildpackStaging StagingBifrost,
	dockerStaging StagingBifrost,
	buildpackTask TaskBifrost,
	lager lager.Logger) http.Handler {
	handler := httprouter.New()

	appHandler := NewAppHandler(bifrost, lager)
	stageHandler := NewStageHandler(buildpackStaging, dockerStaging, lager)
	taskHandler := NewTaskHandler(lager, buildpackTask)

	registerAppsEndpoints(handler, appHandler)
	registerStageEndpoints(handler, stageHandler)
	registerTaskEndpoints(handler, taskHandler)

	return handler
}

func registerAppsEndpoints(handler *httprouter.Router, appHandler *App) {
	handler.GET("/apps", appHandler.List)
	handler.PUT("/apps/:process_guid", appHandler.Desire)
	handler.POST("/apps/:process_guid", appHandler.Update)
	handler.PUT("/apps/:process_guid/:version_guid/stop", appHandler.Stop)
	handler.PUT("/apps/:process_guid/:version_guid/stop/:instance", appHandler.StopInstance)
	handler.GET("/apps/:process_guid/:version_guid/instances", appHandler.GetInstances)
	handler.GET("/apps/:process_guid/:version_guid", appHandler.GetApp)
}

func registerStageEndpoints(handler *httprouter.Router, stageHandler *Stage) {
	handler.POST("/stage/:staging_guid", stageHandler.Stage)
	handler.PUT("/stage/:staging_guid/completed", stageHandler.StagingComplete)
}

func registerTaskEndpoints(handler *httprouter.Router, taskHandler *Task) {
	handler.POST("/tasks/:task_guid", taskHandler.Run)
}
