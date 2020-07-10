package staging_reporter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/bifrost"
	"code.cloudfoundry.org/eirini/integration/util"
	"code.cloudfoundry.org/eirini/k8s"
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

var _ = FDescribe("TaskReporter", func() {
	var (
		cloudControllerServer  *ghttp.Server
		configFile             string
		certPath               string
		keyPath                string
		session                *gexec.Session
		taskDesirer            k8s.TaskDesirer
		task                   *opi.Task
		completionCallbackPath string
	)

	BeforeEach(func() {
		certPath, keyPath = util.GenerateKeyPair("cloud_controller")

		var err error
		cloudControllerServer, err = util.CreateTestServer(certPath, keyPath, certPath)
		Expect(err).ToNot(HaveOccurred())
		completionCallbackPath = "/" + util.Guidify("the-callback")
		cloudControllerServer.Reset()
		// cloudControllerServer.RouteToHandler("POST", completionCallbackPath, ghttp.RespondWith(200, nil))
		cloudControllerServer.RouteToHandler("POST", completionCallbackPath, func(w http.ResponseWriter, r *http.Request) {
			fmt.Printf("=============> handler got request %s\n", prettyPrintRequest(r))
			bodyContent, err := ioutil.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(503)
				return
			}
			r.Body.Close()

			r.Body = cachingReadCloser{cache: bodyContent}

			w.WriteHeader(200)
		})
		// cloudControllerServer.SetAllowUnhandledRequests(true)
		cloudControllerServer.Start()

		config := &eirini.TaskReporterConfig{
			KubeConfig: eirini.KubeConfig{
				Namespace:  fixture.Namespace,
				ConfigPath: fixture.KubeConfigPath,
			},
			CCCertPath: certPath,
			CAPath:     certPath,
			CCKeyPath:  keyPath,
		}

		session, configFile = eiriniBins.TaskReporter.Run(config)

		taskDesirer = k8s.TaskDesirer{
			DefaultStagingNamespace: fixture.Namespace,
			ServiceAccountName:      "",
			JobClient:               k8s.NewJobClient(fixture.Clientset),
			Logger:                  lagertest.NewTestLogger("task-reporter-test"),
			SecretsClient:           k8s.NewSecretsClient(fixture.Clientset),
		}

		task = &opi.Task{
			Image:              "busybox",
			Command:            []string{"echo", "hi"},
			GUID:               util.Guidify("the-task-guid"),
			CompletionCallback: fmt.Sprintf("%s%s", cloudControllerServer.URL(), completionCallbackPath),
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
	})

	JustBeforeEach(func() {
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

	FIt("notifies the cloud controller of a task completion", func() {
		Eventually(cloudControllerServer.ReceivedRequests, "10s").Should(ContainElement(SatisfyAll(
			MatchRequestMethod("POST"),
			MatchRequestPath(completionCallbackPath),
		)))

		Expect(cloudControllerServer.ReceivedRequests()).To(ContainElement(SatisfyAll(
			MatchRequestMethod("POST"),
			MatchRequestPath(completionCallbackPath),
			MatchJSONBody(
				cf.TaskCompletedRequest{
					TaskGUID: task.GUID,
				},
			),
		)))
	})

	It("deletes the job", func() {
		Eventually(getTaskJobsFn("the-task-guid"), "1m").Should(BeEmpty())
	})

	When("a task job fails", func() {
		BeforeEach(func() {
			task.GUID = util.Guidify("failing-task-guid")
			task.Command = []string{"false"}
		})

		It("notifies the cloud controller of a task failure", func() {
			Eventually(cloudControllerServer.ReceivedRequests, "10s").Should(ContainElement(SatisfyAll(
				MatchRequestMethod("POST"),
				MatchRequestPath(completionCallbackPath),
			)))

			Expect(cloudControllerServer.ReceivedRequests()).To(ContainElement(SatisfyAll(
				MatchRequestMethod("POST"),
				MatchRequestPath(completionCallbackPath),
				MatchJSONBody(
					cf.TaskCompletedRequest{
						TaskGUID:      task.GUID,
						Failed:        true,
						FailureReason: "Error",
					},
				),
			)))
		})

		It("deletes the job", func() {
			Eventually(getTaskJobsFn("failing-task-guid"), "20s").Should(BeEmpty())
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
			jobs, err := getTaskJobsFn(task.GUID)()
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
				_, err := fixture.Clientset.CoreV1().Secrets(fixture.Namespace).Get(registrySecretName, metav1.GetOptions{})
				return err
			}, "10s").Should(MatchError(ContainSubstring(`secrets "%s" not found`, registrySecretName)))
		})
	})
})

func getTaskJobsFn(guid string) func() ([]batchv1.Job, error) {
	return func() ([]batchv1.Job, error) {
		jobs, err := fixture.Clientset.BatchV1().Jobs(fixture.Namespace).List(metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s, %s=%s", k8s.LabelSourceType, "TASK", k8s.LabelGUID, guid),
		})
		return jobs.Items, err
	}
}

type requestMethodMatcher struct {
	method string
}

func MatchRequestMethod(method string) *requestMethodMatcher {
	return &requestMethodMatcher{method: method}
}

func (m *requestMethodMatcher) Match(actual interface{}) (bool, error) {
	request, ok := actual.(*http.Request)
	if !ok {
		return false, fmt.Errorf("MatchRequestVerb expects an http.Request")
	}

	return request.Method == m.method, nil
}

func (m *requestMethodMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s to have method %q", prettyPrintRequest(actual), m.method)
}

func (m *requestMethodMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s not to have method %q", prettyPrintRequest(actual), m.method)
}

type requestPathMatcher struct {
	path string
}

func MatchRequestPath(path string) *requestPathMatcher {
	return &requestPathMatcher{path: path}
}

func (m *requestPathMatcher) Match(actual interface{}) (bool, error) {
	request, ok := actual.(*http.Request)
	if !ok {
		return false, fmt.Errorf("MatchRequestPath expects an http.Request")
	}

	return request.URL.Path == m.path, nil
}

func (m *requestPathMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s to have path %q", prettyPrintRequest(actual), m.path)
}

func (m *requestPathMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s not to have path %q", prettyPrintRequest(actual), m.path)
}

type requestJSONBodyMatcher struct {
	body interface{}
}

func MatchJSONBody(body interface{}) *requestJSONBodyMatcher {
	return &requestJSONBodyMatcher{body: body}
}

func (m *requestJSONBodyMatcher) Match(actual interface{}) (bool, error) {
	request, ok := actual.(*http.Request)
	if !ok {
		return false, fmt.Errorf("MatchPathVerb expects an http.Request")
	}

	fmt.Printf("======> reading body for request %s\n", prettyPrintRequest(request))

	requestBytes, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read request body: %w", err)
	}
	defer request.Body.Close()

	expectedBytes, err := json.Marshal(m.body)
	if err != nil {
		return false, fmt.Errorf("failed to marshal ")
	}

	return MatchJSON(expectedBytes).Match(requestBytes)
}

func (m *requestJSONBodyMatcher) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s to have body %v", prettyPrintRequest(actual), m.body)
}

func (m *requestJSONBodyMatcher) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("expected %s not to have body %v", prettyPrintRequest(actual), m.body)
}

func prettyPrintRequest(actual interface{}) string {
	request, ok := actual.(*http.Request)
	if !ok {
		return "prettyPrintRequest expects an http.Request"
	}
	return fmt.Sprintf("/%s %s", request.Method, request.URL.Path)
}

type cachingReadCloser struct {
	cache []byte
}

func (p cachingReadCloser) Read(buf []byte) (n int, err error) {
	return bytes.NewBuffer(p.cache).Read(buf)
}

func (p cachingReadCloser) Close() error {
	return nil
}
