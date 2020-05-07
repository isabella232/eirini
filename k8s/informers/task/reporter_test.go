package task_test

import (
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/k8s/informers/task"
	"code.cloudfoundry.org/lager/lagertest"
)

var _ = Describe("Reporter", func() {

	var (
		reporter task.StateReporter
		server   *ghttp.Server
		logger   *lagertest.TestLogger
		handlers []http.HandlerFunc
		job      *batchv1.Job
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("task-reporter-test")

		handlers = []http.HandlerFunc{
			ghttp.VerifyRequest("PUT", "/task/the-task-guid/completed"),
		}
	})

	JustBeforeEach(func() {
		server = ghttp.NewServer()
		server.AppendHandlers(
			ghttp.CombineHandlers(handlers...),
		)

		reporter = task.StateReporter{
			Client: &http.Client{},
			Logger: logger,
		}

		job = &batchv1.Job{
			ObjectMeta: v1.ObjectMeta{
				Labels: map[string]string{
					k8s.LabelGUID: "the-task-guid",
				},
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Env: []corev1.EnvVar{
									{
										Name:  "EIRINI_ADDRESS",
										Value: server.URL(),
									},
								},
							},
						},
					},
				},
			},
			Status: batchv1.JobStatus{
				Conditions: []batchv1.JobCondition{
					{
						Type: batchv1.JobComplete,
					},
				},
			},
		}
	})

	AfterEach(func() {
		server.Close()
	})

	It("completes the task execution in eirini", func() {
		reporter.Report(job)
		Expect(server.ReceivedRequests()).To(HaveLen(1))
	})

	When("the EIRINI_ADDRESS is not set", func() {
		JustBeforeEach(func() {
			job.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{}
		})

		It("logs the error", func() {
			reporter.Report(job)

			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("task-reporter-test.getting env variable 'EIRINI_ADDRESS' failed"))
			Expect(log.Data).To(HaveKeyWithValue("error", "failed to find env var"))
		})
	})

	When("job has not completed", func() {
		JustBeforeEach(func() {
			job.Status = batchv1.JobStatus{}
		})

		It("doesn't send anything to eirini", func() {
			reporter.Report(job)
			Expect(server.ReceivedRequests()).To(HaveLen(0))
		})
	})

	When("the eirini server returns an unexpected status code", func() {
		BeforeEach(func() {
			handlers = []http.HandlerFunc{
				ghttp.VerifyRequest("PUT", "/task/the-task-guid/completed"),
				ghttp.RespondWith(http.StatusBadGateway, "potato"),
			}
		})

		It("logs the error", func() {
			reporter.Report(job)
			logs := logger.Logs()
			Expect(logs).To(HaveLen(1))
			log := logs[0]
			Expect(log.Message).To(Equal("task-reporter-test.cannot send task status response"))
			Expect(log.Data).To(HaveKeyWithValue("error", "request not successful: status=502 potato"))
		})
	})
})
