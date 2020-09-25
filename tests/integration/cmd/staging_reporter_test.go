package cmd_test

import (
	"os"
	"syscall"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("StagingReporter", func() {
	var (
		config         *eirini.StagingReporterConfig
		configFilePath string
		session        *gexec.Session
	)

	JustBeforeEach(func() {
		session, configFilePath = eiriniBins.StagingReporter.Run(config)
	})

	AfterEach(func() {
		if configFilePath != "" {
			Expect(os.Remove(configFilePath)).To(Succeed())
		}
		if session != nil {
			Eventually(session.Kill()).Should(gexec.Exit())
		}
	})

	Context("When staging reporter is executed with a valid config", func() {
		BeforeEach(func() {
			config = defaultStagingReporterConfig()
		})

		FIt("should be able to start properly", func() {
			Expect(session.Command.Process.Signal(syscall.Signal(0))).To(Succeed())
			Consistently(func() error {
				return session.Command.Process.Signal(syscall.Signal(0))
			}, "5s", "1s").Should(Succeed())
		})
	})
})
