package k8s

import (
	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/opi"
	"k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
)

const ActiveDeadlineSeconds = 900

type TaskDesirer struct {
	Namespace string
	Client    kubernetes.Interface
}

func (d *TaskDesirer) Desire(task *opi.Task) error {
	_, err := d.Client.BatchV1().Jobs(d.Namespace).Create(toJob(task))
	return err
}

func (d *TaskDesirer) Delete(name string) error {
	return d.Client.BatchV1().Jobs(d.Namespace).Delete(name, &meta_v1.DeleteOptions{})
}

func toJob(task *opi.Task) *batch.Job {
	job := &batch.Job{
		Spec: batch.JobSpec{
			ActiveDeadlineSeconds: int64ptr(ActiveDeadlineSeconds),
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{{
						Name:  "opi-task",
						Image: task.Image,
						Env:   MapToEnvVar(task.Env),
					}},
					RestartPolicy: v1.RestartPolicyNever,
				},
			},
		},
	}

	job.Name = task.Env[eirini.EnvStagingGUID]

	job.Spec.Template.Labels = map[string]string{
		"name": task.Env[eirini.EnvAppID],
	}

	job.Labels = map[string]string{
		"name": task.Env[eirini.EnvAppID],
	}
	return job
}
