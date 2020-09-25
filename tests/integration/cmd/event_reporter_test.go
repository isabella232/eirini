package cmd_test

import (
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("EventReporter", func() {
	var (
		config         *eirini.EventReporterConfig
		configFilePath string
		session        *gexec.Session
	)

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.EventsReporter.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	When("event reporter is executed with a valid config", func() {
		BeforeEach(func() {
			config = defaultEventReporterConfig()
		})

		It("should be able to start properly", func() {
			Expect(session.Command.Process.Signal(syscall.Signal(0))).To(Succeed())
		})
	})

	When("event reporter is executed with an empty config", func() {
		BeforeEach(func() {
			config = nil
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("invalid configuration: no configuration has been provided"))
		})
	})

	When("event reporter command with non-existent TLS certs", func() {
		BeforeEach(func() {
			config = defaultEventReporterConfig()
			config.CCCAPath = "/does/not/exist"
			config.CCCertPath = "/does/not/exist"
			config.CCKeyPath = "/does/not/exist"
		})

		It("fails", func() {
			Eventually(session, "10s").Should(gexec.Exit())
			Expect(session.ExitCode()).NotTo(BeZero())
			Expect(session.Err).To(gbytes.Say("failed to load keypair: open /does/not/exist: no such file or directory"))
		})
	})
})
