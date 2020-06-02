package task

import (
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/lager"
	corev1 "k8s.io/api/core/v1"
)

//counterfeiter:generate . Deleter

type Deleter interface {
	Delete(guid string) error
}

type StateReporter struct {
	Client      *http.Client
	Logger      lager.Logger
	TaskDeleter Deleter
}

func (r StateReporter) Report(pod *corev1.Pod) {
	taskGUID := pod.Annotations[k8s.AnnotationGUID]
	uri := pod.Annotations[k8s.AnnotationCompletionCallback]

	taskContainerStatus, ok := getTaskContainerStatus(pod)
	if !ok {
		r.Logger.Error("task container not found", nil, lager.Data{"taskGuid": taskGUID})
		return
	}
	if taskContainerStatus.State.Terminated == nil {
		return
	}

	req := r.generateTaskComletedRequest(taskGUID, taskContainerStatus)

	if err := utils.Post(r.Client, uri, req); err != nil {
		r.Logger.Error("cannot send task status response", err, lager.Data{"taskGuid": taskGUID})
	}

	if err := r.TaskDeleter.Delete(taskGUID); err != nil {
		r.Logger.Error("cannot delete job", err, lager.Data{"taskGuid": taskGUID})
	}
}

func (r StateReporter) generateTaskComletedRequest(guid string, status corev1.ContainerStatus) cf.TaskCompletedRequest {
	res := cf.TaskCompletedRequest{
		TaskGUID: guid,
	}
	terminated := status.State.Terminated

	if terminated.ExitCode != 0 {
		res.Failed = true
		res.FailureReason = terminated.Reason
		r.Logger.Error("job failed", nil, lager.Data{
			"taskGuid":       guid,
			"failureReason":  terminated.Reason,
			"failureMessage": terminated.Message,
		})
	}

	return res
}

func getTaskContainerStatus(pod *corev1.Pod) (corev1.ContainerStatus, bool) {
	taskContainerName := pod.Annotations[k8s.AnnotationOpiTaskContainerName]
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == taskContainerName {
			return status, true
		}
	}
	return corev1.ContainerStatus{}, false
}
