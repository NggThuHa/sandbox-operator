/*
Copyright 2026.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	labv1alpha1 "github.com/ngtukien/systemd-operator/api/v1alpha1"
)

var _ = Describe("VirtualInstance Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-virtualinstance-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		virtualInstance := &labv1alpha1.VirtualInstance{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind VirtualInstance with valid Spec")
			err := k8sClient.Get(ctx, typeNamespacedName, virtualInstance)
			if err != nil && errors.IsNotFound(err) {
				resourceObj := &labv1alpha1.VirtualInstance{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: labv1alpha1.VirtualInstanceSpec{
						VirtualClusterRef: "parent-virtualcluster-mock",
						Image:             "ubuntu:24.04",
						Resources: labv1alpha1.VirtualInstanceResources{
							Storage: labv1alpha1.VirtualInstanceStorageLimit{
								Limit: resource.MustParse("10Gi"),
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resourceObj)).To(Succeed())
			}
		})

		AfterEach(func() {
			resourceObj := &labv1alpha1.VirtualInstance{}
			if err := k8sClient.Get(ctx, typeNamespacedName, resourceObj); err == nil {
				By("Removing finalizers for clean test environment teardown")
				resourceObj.Finalizers = nil
				_ = k8sClient.Update(ctx, resourceObj)

				By("Cleanup the specific resource instance VirtualInstance")
				_ = k8sClient.Delete(ctx, resourceObj)
			}
		})

		It("should successfully reconcile the resource without crashing", func() {
			controllerReconciler := &VirtualInstanceReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling Step 1: Attaching Cross-Namespace Finalizer")
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Reconciling Step 2: Checking parent VirtualCluster status and requeueing cleanly")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
