package lib_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syntasso/kratix/api/v1alpha1"
	"github.com/syntasso/kratix/work-creator/pipeline/lib"
	"github.com/syntasso/kratix/work-creator/pipeline/lib/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Reader", func() {
	var (
		tempDir    string
		fakeClient dynamic.Interface

		subject lib.Reader
		params  *helpers.Parameters
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "health-state-test")
		Expect(err).NotTo(HaveOccurred())

		params = &helpers.Parameters{
			InputDir:        tempDir,
			ObjectGroup:     "group",
			ObjectName:      "name-foo",
			ObjectVersion:   "version",
			ObjectNamespace: "ns-foo",
			CRDPlural:       "thekinds",

			PromiseName:   "test-promise",
			Healthcheck:   false,
			ClusterScoped: false,
		}

		originalGetInputDir := helpers.GetParametersFromEnv
		originalGetK8sClient := helpers.GetK8sClient
		helpers.GetParametersFromEnv = func() *helpers.Parameters {
			return params
		}
		helpers.GetK8sClient = func() (dynamic.Interface, error) {
			return fakeClient, nil
		}
		DeferCleanup(func() {
			helpers.GetParametersFromEnv = originalGetInputDir
			helpers.GetK8sClient = originalGetK8sClient
		})

		promise := v1alpha1.Promise{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "platform.kratix.io/v1alpha1",
				Kind:       "Promise",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-promise",
			},
		}

		scheme := runtime.NewScheme()
		v1alpha1.AddToScheme(scheme)

		fakeClient = fake.NewSimpleDynamicClient(scheme,
			newUnstructured("group/version", "TheKind", "ns-foo", "name-foo"),
			&promise,
		)

		subject = lib.Reader{}
	})

	AfterEach(func() {
		Expect(os.RemoveAll(tempDir)).To(Succeed())
	})

	It("should write the object to a file", func() {
		Expect(subject.Run()).To(Succeed())

		_, err := os.Stat(params.GetObjectPath())
		Expect(err).NotTo(HaveOccurred())

		fileContent, err := os.ReadFile(params.GetObjectPath())
		Expect(err).NotTo(HaveOccurred())

		var object map[string]interface{}
		err = yaml.Unmarshal(fileContent, &object)
		Expect(err).NotTo(HaveOccurred())

		Expect(object["apiVersion"]).To(Equal("group/version"))
		Expect(object["kind"]).To(Equal("TheKind"))
		Expect(object["metadata"].(map[string]interface{})["name"]).To(Equal("name-foo"))
		Expect(object["metadata"].(map[string]interface{})["namespace"]).To(Equal("ns-foo"))
		Expect(object["spec"].(map[string]interface{})["test"]).To(Equal("bar"))
	})

	It("should write the promise to a file if healthcheck is set to true", func() {
		params.Healthcheck = true

		Expect(subject.Run()).To(Succeed())

		_, err := os.Stat(params.GetObjectPath())
		Expect(err).NotTo(HaveOccurred(), "object file should exist")

		_, err = os.Stat(params.GetPromisePath())
		Expect(err).NotTo(HaveOccurred())

		fileContent, err := os.ReadFile(params.GetPromisePath())
		Expect(err).NotTo(HaveOccurred())

		var object map[string]interface{}
		err = yaml.Unmarshal(fileContent, &object)
		Expect(err).NotTo(HaveOccurred())

		Expect(object["apiVersion"]).To(Equal("platform.kratix.io/v1alpha1"))
		Expect(object["kind"]).To(Equal("Promise"))
		Expect(object["metadata"].(map[string]interface{})["name"]).To(Equal("test-promise"))
		Expect(object["metadata"].(map[string]interface{})["namespace"]).To(BeNil())
	})
})

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
			"spec": map[string]interface{}{
				"test": "bar",
			},
		},
	}
}
