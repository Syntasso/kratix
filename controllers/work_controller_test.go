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
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/syntasso/kratix/controllers"
	"github.com/syntasso/kratix/controllers/controllersfakes"
	"github.com/syntasso/kratix/lib/hash"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/syntasso/kratix/api/v1alpha1"
	platformv1alpha1 "github.com/syntasso/kratix/api/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var work *platformv1alpha1.Work

var _ = Describe("WorkReconciler", func() {
	var (
		ctx              context.Context
		reconciler       *controllers.WorkReconciler
		fakeScheduler    *controllersfakes.FakeWorkScheduler
		workName         types.NamespacedName
		work             *v1alpha1.Work
		workResourceName = "work-controller-test-resource-request"
	)

	BeforeEach(func() {
		ctx = context.Background()
		disabled := false
		fakeScheduler = &controllersfakes.FakeWorkScheduler{}
		reconciler = &controllers.WorkReconciler{
			Client:    fakeK8sClient,
			Log:       ctrl.Log.WithName("controllers").WithName("Work"),
			Scheduler: fakeScheduler,
			Disabled:  disabled,
		}

		workName = types.NamespacedName{
			Name:      workResourceName,
			Namespace: "default",
		}
		work = &platformv1alpha1.Work{}
		work.Name = workResourceName
		work.Namespace = "default"
		work.Spec.Replicas = platformv1alpha1.ResourceRequestReplicas
		work.Spec.WorkloadGroups = []platformv1alpha1.WorkloadGroup{
			{
				ID: hash.ComputeHash("."),
				Workloads: []platformv1alpha1.Workload{
					{
						Content: "{someApi: foo, someValue: bar}",
					},
					{
						Content: "{someApi: baz, someValue: bat}",
					},
				},
			},
		}
	})

	When("the resource does not exist", func() {
		It("succeeds and does not requeue", func() {
			result, err := t.reconcileUntilCompletion(reconciler, work)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	When("the scheduler is able to schedule the work successfully", func() {
		BeforeEach(func() {
			fakeScheduler.ReconcileWorkReturns([]string{}, nil)
			Expect(fakeK8sClient.Create(ctx, work)).To(Succeed())
			Expect(fakeK8sClient.Get(ctx, workName, work)).To(Succeed())
		})

		It("succeeds and does not requeue", func() {
			result, err := t.reconcileUntilCompletion(reconciler, work)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))
		})
	})

	When("the scheduler returns an error reconciling the work", func() {
		var schedulingErrorString = "scheduling error"

		BeforeEach(func() {
			fakeScheduler.ReconcileWorkReturns([]string{}, errors.New(schedulingErrorString))
			Expect(fakeK8sClient.Create(ctx, work)).To(Succeed())
			Expect(fakeK8sClient.Get(ctx, workName, work)).To(Succeed())
		})

		It("errors", func() {
			_, err := t.reconcileUntilCompletion(reconciler, work)
			Expect(err).To(MatchError(schedulingErrorString))
		})
	})

	When("the scheduler returns work that could not be scheduled", func() {
		When("the work is a resource request", func() {
			BeforeEach(func() {
				workloadGroupIds := []string{"5058f1af8388633f609cadb75a75dc9d"}
				work.Spec.Replicas = 1
				fakeScheduler.ReconcileWorkReturns(workloadGroupIds, nil)
				Expect(fakeK8sClient.Create(ctx, work)).To(Succeed())
				Expect(fakeK8sClient.Get(ctx, workName, work)).To(Succeed())
			})

			It("re-reconciles until a destinations become available", func() {
				_, err := t.reconcileUntilCompletion(reconciler, work)
				Expect(err).To(MatchError("reconcile loop detected"))

				fakeScheduler.ReconcileWorkReturns(nil, nil)
				result, err := t.reconcileUntilCompletion(reconciler, work)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})

		When("the work is a dependency", func() {
			BeforeEach(func() {
				workloadGroupIds := []string{"5058f1af8388633f609cadb75a75dc9d"}
				work.Spec.Replicas = -1
				fakeScheduler.ReconcileWorkReturns(workloadGroupIds, nil)
				Expect(fakeK8sClient.Create(ctx, work)).To(Succeed())
				Expect(fakeK8sClient.Get(ctx, workName, work)).To(Succeed())

			})

			It("does not requeue", func() {
				result, err := t.reconcileUntilCompletion(reconciler, work)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))
			})
		})
	})
})
