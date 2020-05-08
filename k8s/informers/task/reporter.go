package task

import (
	"fmt"
	"net/http"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/lager"
	batchv1 "k8s.io/api/batch/v1"
)

type StateReporter struct {
	Client *http.Client
	Logger lager.Logger
}

func (r StateReporter) Report(job *batchv1.Job) {
	eiriniAddr, err := utils.GetEnvVarValue("EIRINI_ADDRESS", job.Spec.Template.Spec.Containers[0].Env)
	if err != nil {
		r.Logger.Error("getting env variable 'EIRINI_ADDRESS' failed", err)
		return
	}

	if len(job.Status.Conditions) != 0 {
		taskGUID := job.Labels[k8s.LabelGUID]
		uri := fmt.Sprintf("%s/tasks/%s/completed", eiriniAddr, taskGUID)
		if err := utils.Put(r.Client, uri, nil); err != nil {
			r.Logger.Error("cannot send task status response", err, lager.Data{"taskGuid": taskGUID})
			return
		}
	}
}
