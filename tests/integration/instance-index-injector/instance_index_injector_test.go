package instance_index_injector_test

import (
	"context"
	"fmt"
	"os"

	"code.cloudfoundry.org/eirini"
	"code.cloudfoundry.org/eirini/k8s"
	"code.cloudfoundry.org/eirini/tests"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexInjector", func() {
	const (
		// as defined in scripts/assets/kinda-run-tests/test-job.yml
		serviceNamespace = "eirini-test"
		testPodName      = "eirini-test"
	)

	var (
		config         *eirini.InstanceIndexEnvInjectorConfig
		configFilePath string
		hookSession    *gexec.Session
		pod            *corev1.Pod
		serviceName    string
	)

	BeforeEach(func() {
		guid := tests.GenerateGUID()[:8]
		port := int32(10000 + GinkgoParallelNode() - 1)
		serviceName = fmt.Sprintf("env-injector-%d-%s", GinkgoParallelNode(), guid)

		// create service that routes to the env injector server created by eirinix
		tests.CreateService(fixture.Clientset, serviceNamespace, serviceName, map[string]string{"name": testPodName}, port)

		// config & run env injector binary that uses eirinix to create a server and mutating webhook
		config = &eirini.InstanceIndexEnvInjectorConfig{
			ServiceName:                serviceName,
			ServicePort:                port,
			ServiceNamespace:           serviceNamespace,
			EiriniXOperatorFingerprint: serviceName,
			WorkloadsNamespace:         fixture.Namespace,
			KubeConfig: eirini.KubeConfig{
				ConfigPath: fixture.KubeConfigPath,
			},
		}
		hookSession, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)

		tests.WaitForServiceReadiness(serviceNamespace, serviceName, port, "/0", true)

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name-0",
				Labels: map[string]string{
					k8s.LabelSourceType: "APP",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  k8s.OPIContainerName,
						Image: "eirini/dorini",
					},
					{
						Name:  "not-opi",
						Image: "eirini/dorini",
					},
				},
			},
		}
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if hookSession != nil {
			Eventually(hookSession.Kill()).Should(gexec.Exit())
		}
		tests.DeleteService(fixture.Clientset, serviceNamespace, serviceName)

		// cleanup artifacts created by eirinix
		err := fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), serviceName+"-mutating-hook", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
		err = fixture.Clientset.CoreV1().Secrets(serviceNamespace).Delete(context.Background(), serviceName+"-setupcertificate", metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		pod, err = fixture.Clientset.CoreV1().Pods(fixture.Namespace).Create(context.Background(), pod, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})

	getCFInstanceIndex := func(pod *corev1.Pod, containerName string) string {
		for _, container := range pod.Spec.Containers {
			if container.Name != containerName {
				continue
			}

			for _, e := range container.Env {
				if e.Name != eirini.EnvCFInstanceIndex {
					continue
				}

				return e.Value
			}
		}

		return ""
	}

	It("sets CF_INSTANCE_INDEX in the opi container environment", func() {
		Expect(getCFInstanceIndex(pod, k8s.OPIContainerName)).To(Equal("0"))
	})

	It("does not set CF_INSTANCE_INDEX on the non-opi container", func() {
		Expect(getCFInstanceIndex(pod, "not-opi")).To(Equal(""))
	})
})
