package eirini_controller_test

import (
	"fmt"

	"code.cloudfoundry.org/eirini"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	// "code.cloudfoundry.org/eirini/integration/eirini-controller"
)

var _ = Describe("PodCrash", func() {
	var (
		config  eirini.Config
		session *gexec.Session
	)

	BeforeEach(func() {
		config = eirini.Config{
			Properties: eirini.Properties{
				KubeConfig: eirini.KubeConfig{
					// TODO: env var here?
					ConfigPath: "/home/vagrant/.kube/config",
				},
			},
		}

		session, _ = eiriniBins.EiriniController.Run(config)
		fmt.Printf("session = %+v\n", session)
	})

	AfterEach(func() {
		session.Kill()
	})

	It("does something", func() {
		Eventually(session).Should(gbytes.Say("foo"))
	})
})
