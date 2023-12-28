/*
Copyright 2021 Syntasso.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers_test

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"

	fakeclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	fakeK8sClient           client.Client
	fakeApiExtensionsClient apiextensionsv1.CustomResourceDefinitionsGetter
	apiextensionClient      apiextensionsv1.CustomResourceDefinitionsGetter
	testEnv                 *envtest.Environment
	k8sManager              ctrl.Manager
	t                       *testReconciler

	timeout             = "30s"
	consistentlyTimeout = "6s"
	interval            = "3s"
)

var _ = BeforeSuite(func(_ SpecContext) {
	err := platformv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

}, NodeTimeout(time.Minute))

var _ = AfterSuite(func() {
})

var _ = BeforeEach(func() {
	yamlFile, err := os.ReadFile(resourceRequestPath)
	Expect(err).ToNot(HaveOccurred())

	resReq := &unstructured.Unstructured{}
	Expect(yaml.Unmarshal(yamlFile, resReq)).To(Succeed())

	fakeK8sClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithStatusSubresource(
		&platformv1alpha1.PromiseRelease{},
		&platformv1alpha1.Promise{},
		&platformv1alpha1.Work{},
		&platformv1alpha1.WorkPlacement{},
		&platformv1alpha1.Destination{},
		&platformv1alpha1.GitStateStore{},
		&platformv1alpha1.BucketStateStore{},
		//Add redis.marketplace.kratix.io/v1alpha1 so we can update its status
		resReq,
	).Build()

	fakeApiExtensionsClient = fakeclientset.NewSimpleClientset().ApiextensionsV1()
	t = &testReconciler{}
})

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}
