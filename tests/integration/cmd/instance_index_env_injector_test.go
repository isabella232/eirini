package cmd_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("InstanceIndexEnvInjector", func() {
	var (
		config         *eirini.InstanceIndexEnvInjectorConfig
		configFilePath string
		session        *gexec.Session
	)

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.InstanceIndexEnvInjector.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	When("the webhook is executed with a valid config", func() {
		BeforeEach(func() {
			config = defaultInstanceIndexEnvInjectorConfig()
		})

		AfterEach(func() {
			Expect(fixture.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.Background(), "cmd-test-mutating-hook", metav1.DeleteOptions{})).To(Succeed())
		})

		It("starts properly", func() {
			Expect(session.Command.Process.Signal(syscall.Signal(0))).To(Succeed())
			Eventually(func() error {
				_, err := net.Dial("tcp", fmt.Sprintf(":%d", config.ServicePort))

				return err
			}, "5s").Should(Succeed())
		})
	})

	When("the webhook is executed with an empty config", func() {
		BeforeEach(func() {
			config = nil
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("setting up the webhook server certificate: an empty namespace may not be set when a resource name is provided"))
		})
	})
})
