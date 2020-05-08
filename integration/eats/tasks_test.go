package eats_test

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/models/cf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = FDescribe("Tasks", func() {
	var (
		taskReporterConfigFile string
		taskReporterSession    *gexec.Session

		certPath string
		keyPath  string

		task cf.TaskRequest
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("opi")

		config := &eirini.ReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			EiriniCertPath: certPath,
			CAPath:         certPath,
			EiriniKeyPath:  keyPath,
		}

		taskReporterSession, taskReporterConfigFile = util.RunBinary(binPaths.TaskReporter, config)
	})

	AfterEach(func() {
		if taskReporterSession != nil {
			taskReporterSession.Kill()
		}
		Expect(os.Remove(taskReporterConfigFile)).To(Succeed())
		Expect(os.Remove(certPath)).To(Succeed())
		Expect(os.Remove(keyPath)).To(Succeed())
	})

	Context("When an task is created", func() {
		BeforeEach(func() {
			task = cf.TaskRequest{
				GUID: "the-task",
				Lifecycle: cf.Lifecycle{
					BuildpackLifecycle: &cf.BuildpackLifecycle{
						StartCommand: "fubar",
					},
				},
			}
			resp, err := desireTask(httpClient, opiURL, task)
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))
		})

		It("cleans up the job after it completes", func() {
			time.Sleep(5 * time.Second)
			jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s, %s=%s", k8s.LabelSourceType, "TASK", k8s.LabelGUID, "the-task"),
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs.Items).To(BeEmpty())
		})
	})
})
