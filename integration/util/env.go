package util

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
)

const DefaultApplicationServiceAccount = "eirini"

func GetKubeconfig() string {
	kubeconfPath := os.Getenv("INTEGRATION_KUBECONFIG")
	if kubeconfPath != "" {
		return kubeconfPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		Fail("INTEGRATION_KUBECONFIG not provided, failed to use default: " + err.Error())
	}

	kubeconfPath = filepath.Join(homeDir, ".kube", "config")

	_, err = os.Stat(kubeconfPath)
	if os.IsNotExist(err) {
		kubeconf, err := clientcmd.BuildConfigFromFlags("", "")
		Expect(err).NotTo(HaveOccurred())
		kubeConfBytes, err := yaml.Marshal(kubeconf)
		Expect(err).NotTo(HaveOccurred())
		Expect(ioutil.WriteFile(kubeconfPath, kubeConfBytes, 0o755)).To(Succeed())
	} else {
		Expect(err).NotTo(HaveOccurred())
	}

	return kubeconfPath
}

func GetEiriniDockerHubPassword() string {
	password := os.Getenv("EIRINIUSER_PASSWORD")
	if password == "" {
		Skip("eiriniuser password not provided. Please export EIRINIUSER_PASSWORD")
	}

	return password
}

func GetApplicationServiceAccount() string {
	serviceAccountName := os.Getenv("APPLICATION_SERVICE_ACCOUNT")
	if serviceAccountName != "" {
		return serviceAccountName
	}

	return DefaultApplicationServiceAccount
}

func IsUsingDeployedEirini() bool {
	_, set := os.LookupEnv("USE_DEPLOYED_EIRINI")

	return set
}
