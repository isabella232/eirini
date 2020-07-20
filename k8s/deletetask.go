package k8s

import (
	"fmt"
	"strings"

	"code.cloudfoundry.org/eirini/k8s/utils"
	"code.cloudfoundry.org/lager"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batch "k8s.io/api/batch/v1"
)

//counterfeiter:generate . JobListerDeleter
type JobListerDeleter interface {
	List(opts meta_v1.ListOptions) (*batch.JobList, error)
	Delete(namespace string, name string, options meta_v1.DeleteOptions) error
}

//counterfeiter:generate . SecretsDeleter
type SecretsDeleter interface {
	Delete(namespace, name string) error
}

type TaskDeleter struct {
	logger         lager.Logger
	jobClient      JobListerDeleter
	secretsDeleter SecretsDeleter
}

func NewTaskDeleter(
	logger lager.Logger,
	jobClient JobListerDeleter,
	secretsDeleter SecretsDeleter,
) *TaskDeleter {
	return &TaskDeleter{
		logger:         logger,
		jobClient:      jobClient,
		secretsDeleter: secretsDeleter,
	}
}

func (d *TaskDeleter) Delete(guid string) (string, error) {
	return d.delete(guid, LabelGUID)
}

func (d *TaskDeleter) DeleteStaging(guid string) error {
	_, err := d.delete(guid, LabelStagingGUID)
	return err
}

func (d *TaskDeleter) delete(guid, label string) (string, error) {
	logger := d.logger.Session("delete", lager.Data{"guid": guid})
	jobs, err := d.jobClient.List(meta_v1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", label, guid),
	})
	if err != nil {
		logger.Error("failed to list jobs", err)
		return "", err
	}
	if len(jobs.Items) != 1 {
		logger.Error("job with guid does not have 1 instance", nil, lager.Data{"instances": len(jobs.Items)})
		return "", fmt.Errorf("job with guid %s should have 1 instance, but it has: %d", guid, len(jobs.Items))
	}

	job := jobs.Items[0]
	if err := d.deleteDockerRegistrySecret(job); err != nil {
		return "", err
	}

	callbackURL := job.Annotations[AnnotationCompletionCallback]
	backgroundPropagation := meta_v1.DeletePropagationBackground
	return callbackURL, d.jobClient.Delete(job.Namespace, job.Name, meta_v1.DeleteOptions{
		PropagationPolicy: &backgroundPropagation,
	})
}

func (d *TaskDeleter) deleteDockerRegistrySecret(job batch.Job) error {
	dockerSecretNamePrefix := dockerImagePullSecretNamePrefix(
		job.Annotations[AnnotationAppName],
		job.Annotations[AnnotationSpaceName],
		job.Labels[LabelGUID],
	)

	for _, secret := range job.Spec.Template.Spec.ImagePullSecrets {
		if !strings.HasPrefix(secret.Name, dockerSecretNamePrefix) {
			continue
		}
		if err := d.secretsDeleter.Delete(job.Namespace, secret.Name); err != nil {
			return err
		}
	}

	return nil
}

func dockerImagePullSecretNamePrefix(appName, spaceName, taskGUID string) string {
	secretNamePrefix := fmt.Sprintf("%s-%s", appName, spaceName)
	return fmt.Sprintf("%s-registry-secret-", utils.SanitizeName(secretNamePrefix, taskGUID))
}