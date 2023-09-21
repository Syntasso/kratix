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
	"path/filepath"
	"testing"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"
	. "github.com/syntasso/kratix/controllers"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient          client.Client
	apiextensionClient *clientset.Clientset
	testEnv            *envtest.Environment
	k8sManager         ctrl.Manager

	timeout             = "30s"
	consistentlyTimeout = "6s"
	interval            = "3s"
)
var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = platformv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	//+kubebuilder:scaffold:scheme

	apiextensionClient = clientset.NewForConfigOrDie(cfg)
	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&PromiseReconciler{
		Manager:             k8sManager,
		ApiextensionsClient: apiextensionClient,
		Client:              k8sManager.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("PromiseReconciler"),
		RestartManager:      func() {},
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	ns := &v1.Namespace{
		ObjectMeta: ctrl.ObjectMeta{
			Name: platformv1alpha1.KratixSystemNamespace,
		},
	}
	Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	testEnv.Stop()
})

func cleanEnvironment() {
	var promises platformv1alpha1.PromiseList
	k8sClient.List(context.Background(), &promises)
	for _, p := range promises.Items {
		p.Finalizers = nil
		k8sClient.Update(context.Background(), &p)
	}
	Expect(k8sClient.DeleteAllOf(context.Background(), &platformv1alpha1.Promise{})).To(Succeed())
	Expect(k8sClient.DeleteAllOf(context.Background(), &platformv1alpha1.Destination{})).To(Succeed())

	deleteInNamespace(&platformv1alpha1.Work{}, "default")
	deleteInNamespace(&platformv1alpha1.Work{}, platformv1alpha1.KratixSystemNamespace)

	var workPlacements platformv1alpha1.WorkPlacementList
	k8sClient.List(context.Background(), &workPlacements)
	for _, wp := range workPlacements.Items {
		wp.Finalizers = nil
		k8sClient.Update(context.Background(), &wp)
	}
	deleteInNamespace(&platformv1alpha1.WorkPlacement{}, "default")
	deleteInNamespace(&platformv1alpha1.WorkPlacement{}, platformv1alpha1.KratixSystemNamespace)
}

func deleteInNamespace(obj client.Object, namespace string) {
	Expect(k8sClient.DeleteAllOf(context.Background(), obj, client.InNamespace(namespace))).To(Succeed())
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{})
}
