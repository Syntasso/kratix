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
	"context"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syntasso/kratix/controllers"
	"github.com/syntasso/kratix/lib/hash"
	"github.com/syntasso/kratix/lib/writers"
	"github.com/syntasso/kratix/lib/writers/writersfakes"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/syntasso/kratix/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var workplacement *v1alpha1.Work

var _ = Describe("WorkplacementReconciler", func() {
	var (
		ctx               context.Context
		workloads         []v1alpha1.Workload
		workplacementName = "test-workplacement"
		workPlacement     v1alpha1.WorkPlacement
		reconciler        *controllers.WorkPlacementReconciler
		fakeWriter        *writersfakes.FakeStateStoreWriter
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &controllers.WorkPlacementReconciler{
			Client: fakeK8sClient,
			Log:    ctrl.Log.WithName("controllers").WithName("Work"),
		}

		workloads = []v1alpha1.Workload{
			{
				Content: "{someApi: foo, someValue: bar}",
			},
		}
		workPlacement = v1alpha1.WorkPlacement{
			TypeMeta: v1.TypeMeta{
				Kind:       "WorkPlacement",
				APIVersion: "platform.kratix.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      workplacementName,
				Namespace: "default",
			},
			Spec: v1alpha1.WorkPlacementSpec{
				TargetDestinationName: "test-destination",
				ID:                    hash.ComputeHash("."),
				Workloads:             workloads,
			},
		}
		Expect(fakeK8sClient.Create(ctx, &workPlacement)).To(Succeed())

		destination := &v1alpha1.Destination{
			TypeMeta: v1.TypeMeta{
				Kind:       "Destination",
				APIVersion: "platform.kratix.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: "test-destination",
			},
			Spec: v1alpha1.DestinationSpec{
				FilepathExpression: v1alpha1.FilepathExpression{
					Type: v1alpha1.FilepathExpressionTypeNone,
				},
				StateStoreRef: &v1alpha1.StateStoreReference{
					Kind: "BucketStateStore",
					Name: "test-state-store",
				},
			},
		}
		Expect(fakeK8sClient.Create(ctx, destination)).To(Succeed())

		stateStore := &v1alpha1.BucketStateStore{
			TypeMeta: v1.TypeMeta{
				Kind:       "BucketStateStore",
				APIVersion: "platform.kratix.io/v1alpha1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name: "test-state-store",
			},
			Spec: v1alpha1.BucketStateStoreSpec{
				BucketName: "test-bucket",
				StateStoreCoreFields: v1alpha1.StateStoreCoreFields{
					SecretRef: &corev1.SecretReference{
						Name:      "test-secret",
						Namespace: "default",
					},
				},
				Endpoint: "localhost:9000",
			},
		}
		Expect(fakeK8sClient.Create(ctx, stateStore)).To(Succeed())

		secret := &corev1.Secret{
			TypeMeta: v1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
			},
			Data: map[string][]byte{
				"accessKeyID":     []byte("test-access"),
				"secretAccessKey": []byte("test-secret"),
			},
		}
		Expect(fakeK8sClient.Create(ctx, secret)).To(Succeed())

		fakeWriter = &writersfakes.FakeStateStoreWriter{}
		controllers.SetNewS3Writer(func(logger logr.Logger, stateStoreSpec v1alpha1.BucketStateStoreSpec, destination v1alpha1.Destination,
			creds map[string][]byte) (writers.StateStoreWriter, error) {
			return fakeWriter, nil
		})
	})

	When("the destination has filepathExpression type none", func() {
		It("calls the writer with an empty dir", func() {
			result, err := t.reconcileUntilCompletion(reconciler, &workPlacement)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			Expect(fakeWriter.WriteDirWithObjectsCallCount()).To(Equal(1))
			deleteExistingDir, dir, workloadsToCreate := fakeWriter.WriteDirWithObjectsArgsForCall(0)
			Expect(deleteExistingDir).To(BeTrue())
			Expect(dir).To(Equal(""))
			Expect(workloadsToCreate).To(Equal(workloads))
		})

		It("sets the finalizer", func() {
			result, err := t.reconcileUntilCompletion(reconciler, &workPlacement)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
			workplacement := &v1alpha1.WorkPlacement{}
			Expect(fakeK8sClient.Get(ctx, types.NamespacedName{Name: workplacementName, Namespace: "default"}, workplacement)).
				To(Succeed())
			Expect(workplacement.GetFinalizers()).To(ContainElement("finalizers.workplacement.kratix.io/repo-cleanup"))
		})
	})

	// When("the destination has filepathExpression type nested", func() {
	// })
})
