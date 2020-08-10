package util

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"sync"

	"code.cloudfoundry.org/eirini"
	eiriniclient "code.cloudfoundry.org/eirini/pkg/generated/clientset/versioned"
	"github.com/hashicorp/go-multierror"

	// nolint:golint,stylecheck
	. "github.com/onsi/ginkgo"

	// nolint:golint,stylecheck
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	basePortNumber = 20000
	portRange      = 1000
)

type Fixture struct {
	Clientset         kubernetes.Interface
	EiriniClientset   eiriniclient.Interface
	Namespace         string
	DefaultNamespace  string
	PspName           string
	KubeConfigPath    string
	Writer            io.Writer
	nextAvailablePort int
	portMux           *sync.Mutex
}

func NewFixture(writer io.Writer) *Fixture {
	kubeConfigPath := GetKubeconfig()

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	Expect(err).NotTo(HaveOccurred(), "failed to build config from flags")

	clientset, err := kubernetes.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to create clientset")

	lrpclientset, err := eiriniclient.NewForConfig(config)
	Expect(err).NotTo(HaveOccurred(), "failed to create clientset")

	return &Fixture{
		KubeConfigPath:    kubeConfigPath,
		Clientset:         clientset,
		EiriniClientset:   lrpclientset,
		Writer:            writer,
		nextAvailablePort: basePortNumber + portRange*GinkgoParallelNode(),
		portMux:           &sync.Mutex{},
	}
}

func (f *Fixture) SetUp() {
	if IsUsingDeployedEirini() {
		f.DefaultNamespace = f.getApplicationNamespace()
	} else {
		f.DefaultNamespace = f.configureNewNamespace()
	}

	f.Namespace = f.configureNewNamespace()
}

func (f *Fixture) NextAvailablePort() int {
	f.portMux.Lock()
	defer f.portMux.Unlock()

	if f.nextAvailablePort > f.maxPortNumber() {
		Fail("Ginkgo node %d is not allowed to allocate more than %d ports", GinkgoParallelNode(), portRange)
	}

	port := f.nextAvailablePort
	f.nextAvailablePort++

	return port
}

func (f Fixture) maxPortNumber() int {
	return basePortNumber + portRange*GinkgoParallelNode() + portRange
}

func (f Fixture) DownloadEiriniCertificates() (string, string) {
	certFile, err := ioutil.TempFile("", "cert-")
	Expect(err).NotTo(HaveOccurred())

	defer certFile.Close()

	_, err = certFile.WriteString(f.getSecret("eirini-certs", "tls.crt"))
	Expect(err).NotTo(HaveOccurred())

	keyFile, err := ioutil.TempFile("", "key-")
	Expect(err).NotTo(HaveOccurred())

	defer keyFile.Close()

	_, err = keyFile.WriteString(f.getSecret("eirini-certs", "tls.key"))
	Expect(err).NotTo(HaveOccurred())

	return certFile.Name(), keyFile.Name()
}

func (f *Fixture) TearDown() {
	var errs *multierror.Error
	errs = multierror.Append(errs, f.printDebugInfo())
	errs = multierror.Append(errs, f.deleteNamespace(f.Namespace))

	if !IsUsingDeployedEirini() {
		errs = multierror.Append(errs, f.deleteNamespace(f.DefaultNamespace))
	}

	Expect(errs.ErrorOrNil()).NotTo(HaveOccurred())
}

func (f Fixture) getApplicationNamespace() string {
	cm, err := f.Clientset.CoreV1().ConfigMaps("eirini-core").Get(context.Background(), "eirini", metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	opiYml := cm.Data["opi.yml"]
	config := eirini.Config{}

	Expect(yaml.Unmarshal([]byte(opiYml), &config)).To(Succeed())

	return config.Properties.Namespace
}

func (f Fixture) getSecret(secretName, secretPath string) string {
	secret, err := f.Clientset.CoreV1().Secrets("eirini-core").Get(context.Background(), secretName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	data := secret.Data[secretPath]

	decodedBytes, err := base64.StdEncoding.DecodeString(string(data))
	Expect(err).NotTo(HaveOccurred())

	return string(decodedBytes)
}

func (f *Fixture) configureNewNamespace() string {
	namespace := CreateRandomNamespace(f.Clientset)
	Expect(CreatePodCreationPSP(namespace, getPspName(namespace), GetApplicationServiceAccount(), f.Clientset)).To(Succeed(), "failed to create pod creation PSP")

	return namespace
}

func (f *Fixture) deleteNamespace(namespace string) error {
	var errs *multierror.Error
	errs = multierror.Append(errs, DeleteNamespace(namespace, f.Clientset))
	errs = multierror.Append(errs, DeletePSP(getPspName(namespace), f.Clientset))

	return errs.ErrorOrNil()
}

//nolint:gocyclo
func (f *Fixture) printDebugInfo() error {
	if _, err := f.Writer.Write([]byte("Jobs:\n")); err != nil {
		return err
	}

	jobs, _ := f.Clientset.BatchV1().Jobs(f.Namespace).List(context.Background(), metav1.ListOptions{})

	for _, job := range jobs.Items {
		fmt.Fprintf(f.Writer, "Job: %s status is: %#v\n", job.Name, job.Status)

		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}
	}

	statefulsets, _ := f.Clientset.AppsV1().StatefulSets(f.Namespace).List(context.Background(), metav1.ListOptions{})

	if _, err := f.Writer.Write([]byte("StatefulSets:\n")); err != nil {
		return err
	}

	for _, s := range statefulsets.Items {
		fmt.Fprintf(f.Writer, "StatefulSet: %s status is: %#v\n", s.Name, s.Status)

		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}
	}

	pods, _ := f.Clientset.CoreV1().Pods(f.Namespace).List(context.Background(), metav1.ListOptions{})

	if _, err := f.Writer.Write([]byte("Pods:\n")); err != nil {
		return err
	}

	for _, p := range pods.Items {
		fmt.Fprintf(f.Writer, "Pod: %s status is: %#v\n", p.Name, p.Status)

		if _, err := f.Writer.Write([]byte("-----------\n")); err != nil {
			return err
		}

		fmt.Fprintf(f.Writer, "Pod: %s logs are: \n", p.Name)
		logsReq := f.Clientset.CoreV1().Pods(f.Namespace).GetLogs(p.Name, &corev1.PodLogOptions{})

		if err := consumeRequest(logsReq, f.Writer); err != nil {
			fmt.Fprintf(f.Writer, "Failed to get logs for Pod: %s becase: %v \n", p.Name, err)
		}
	}

	return nil
}

func consumeRequest(request rest.ResponseWrapper, out io.Writer) error {
	readCloser, err := request.Stream(context.Background())
	if err != nil {
		return err
	}
	defer readCloser.Close()

	r := bufio.NewReader(readCloser)

	for {
		bytes, err := r.ReadBytes('\n')
		if _, writeErr := out.Write(bytes); writeErr != nil {
			return writeErr
		}

		if err != nil {
			if err != io.EOF {
				return err
			}

			return nil
		}
	}
}

func getPspName(namespace string) string {
	return fmt.Sprintf("%s-psp", namespace)
}
