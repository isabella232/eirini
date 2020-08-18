package staging_reporter_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/client"
	"code.cloudfoundry.org/eirini/models/cf"
	"code.cloudfoundry.org/eirini/opi"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/onsi/gomega/ghttp"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("TaskReporter", func() {
	var (
		cloudControllerServer *ghttp.Server
		handlers              []http.HandlerFunc
		configFile            string
		certPath              string
		keyPath               string
		session               *gexec.Session
		taskDesirer           *k8s.TaskDesirer
		task                  *opi.Task
		config                *eirini.TaskReporterConfig
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("cloud_controller")

		var err error
		cloudControllerServer, err = util.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		cloudControllerServer.HTTPTestServer.StartTLS()

		eiriniInstance := fmt.Sprintf("%s-%d", util.GenerateGUID(), GinkgoParallelNode())
		config = &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CCCertPath:     certPath,
			CAPath:         certPath,
			CCKeyPath:      keyPath,
			EiriniInstance: eiriniInstance,
		}

		taskDesirer = k8s.NewTaskDesirerWithEiriniInstance(
			lagertest.NewTestLogger("test-task-desirer"),
			client.NewJob(fixture.Clientset, eiriniInstance),
			client.NewSecret(fixture.Clientset),
			fixture.Namespace,
			nil,
			"",
			"",
			"",
			"",
			eiriniInstance)

		taskGUID := util.GenerateGUID()
		task = &opi.Task{
			Image:              "busybox",
			Command:            []string{"echo", "hi"},
			GUID:               taskGUID,
			CompletionCallback: fmt.Sprintf("%s/the-callback", cloudControllerServer.URL()),
			AppName:            "app",
			AppGUID:            "app-guid",
			OrgName:            "org-name",
			OrgGUID:            "org-guid",
			SpaceName:          "space-name",
			SpaceGUID:          "space-guid",
			MemoryMB:           200,
			DiskMB:             200,
			CPUWeight:          1,
		}

		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("POST", "/the-callback"),
			ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{TaskGUID: taskGUID}),
		}
	})

	JustBeforeEach(func() {
		cloudControllerServer.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		session, configFile = eiriniBins.TaskReporter.Run(config)
		Expect(taskDesirer.Desire(fixture.Namespace, task)).To(Succeed())
	})

	AfterEach(func() {
		if session != nil {
			session.Kill()
		}
		Expect(os.Remove(configFile)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		cloudControllerServer.Close()
	})

	It("notifies the cloud controller of a task completion", func() {
		Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		Consistently(cloudControllerServer.ReceivedRequests, "1m").Should(HaveLen(1))
	})

	When("the Cloud Controller is not using TLS", func() {
		BeforeEach(func() {
			config.CCTLSDisabled = true
			config.CCCertPath = ""
			config.CCKeyPath = ""
			config.CAPath = ""
			cloudControllerServer.Close()
			cloudControllerServer = ghttp.NewServer()
			task.CompletionCallback = fmt.Sprintf("%s/the-callback", cloudControllerServer.URL())
		})

		It("still gets notified", func() {
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
		})
	})

	It("deletes the job", func() {
		Eventually(getTaskJobsFn(task.GUID, config.EiriniInstance)).Should(BeEmpty())
	})

	When("a task job fails", func() {
		BeforeEach(func() {
			task.Command = []string{"false"}

			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("POST", "/the-callback"),
				ghttp.VerifyJSONRepresenting(cf.TaskCompletedRequest{
					TaskGUID:      task.GUID,
					Failed:        true,
					FailureReason: "Error",
				}),
			}
		})

		It("notifies the cloud controller of a task failure", func() {
			Eventually(cloudControllerServer.ReceivedRequests).Should(HaveLen(1))
			Consistently(cloudControllerServer.ReceivedRequests, "10s").Should(HaveLen(1))
		})

		It("deletes the job", func() {
			Eventually(getTaskJobsFn(task.GUID, config.EiriniInstance)).Should(BeEmpty())
		})
	})

	When("a private docker registry is used", func() {
		BeforeEach(func() {
			task.Image = "eiriniuser/notdora"
			task.PrivateRegistry = &opi.PrivateRegistry{
				Server:   bifrost.DockerHubHost,
				Username: "eiriniuser",
				Password: util.GetEiriniDockerHubPassword(),
			}
			task.Command = []string{"sleep", "1"}
		})

		It("deletes the docker registry secret", func() {
			registrySecretPrefix := fmt.Sprintf("%s-%s-registry-secret-", task.AppName, task.SpaceName)
			jobs, err := getTaskJobsFn(task.GUID, config.EiriniInstance)()
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs).To(HaveLen(1))

			imagePullSecrets := jobs[0].Spec.Template.Spec.ImagePullSecrets
			var registrySecretName string
			for _, imagePullSecret := range imagePullSecrets {
				if strings.HasPrefix(imagePullSecret.Name, registrySecretPrefix) {
					registrySecretName = imagePullSecret.Name

					break
				}
			}
			Expect(registrySecretName).NotTo(BeEmpty())

			Eventually(func() error {
				_, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(context.Background(), registrySecretName, metav1.GetOptions{})

				return err
			}).Should(MatchError(ContainSubstring(`secrets "%s" not found`, registrySecretName)))
		})
	})

	When("the job is labeled with a different eirini instance ID", func() {
		var desirerEiriniInstance string

		BeforeEach(func() {
			desirerEiriniInstance = config.EiriniInstance
			config.EiriniInstance = "your-eirini" + util.GenerateGUID()
		})

		It("does not notify the cloud controller", func() {
			Consistently(cloudControllerServer.ReceivedRequests, "10s").Should(BeEmpty())
		})

		It("does not delete the task", func() {
			Consistently(getTaskJobsFn(task.GUID, desirerEiriniInstance), "10s").ShouldNot(BeEmpty())
		})
	})
})

func getTaskJobsFn(guid, eiriniInstance string) func() ([]batchv1.Job, error) {
	return func() ([]batchv1.Job, error) {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: fmt.Sprintf(
				"%s=%s, %s=%s, %s=%s",
				k8s.LabelSourceType, "TASK",
				k8s.LabelGUID, guid,
				k8s.LabelEiriniInstance, eiriniInstance,
			),
		})

		return jobs.Items, err
	}
}
