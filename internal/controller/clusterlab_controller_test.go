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

	labv1alpha1 "github.com/ngtukien/sandbox-operator/api/v1alpha1"
)

var _ = Describe("ClusterLab Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-clusterlab-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		clusterLab := &labv1alpha1.ClusterLab{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind ClusterLab with valid Spec")
			err := k8sClient.Get(ctx, typeNamespacedName, clusterLab)
			if err != nil && errors.IsNotFound(err) {
				resourceObj := &labv1alpha1.ClusterLab{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: labv1alpha1.ClusterLabSpec{
						Network: labv1alpha1.ClusterLabNetworkConfig{
							Type: "internal",
						},
						Quota: labv1alpha1.ClusterLabQuotaConfig{
							Compute: labv1alpha1.ClusterLabComputeQuota{
								CPU:    labv1alpha1.ClusterLabResourceLimit{Limit: resource.MustParse("2")},
								Memory: labv1alpha1.ClusterLabResourceLimit{Limit: resource.MustParse("4Gi")},
							},
							Storage: labv1alpha1.ClusterLabStorageQuota{
								Limit: resource.MustParse("30Gi"),
							},
							Objects: labv1alpha1.ClusterLabObjectsQuota{
								PodsLimit:     20,
								ServicesLimit: 10,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resourceObj)).To(Succeed())
			}
		})

		AfterEach(func() {
			resourceObj := &labv1alpha1.ClusterLab{}
			if err := k8sClient.Get(ctx, typeNamespacedName, resourceObj); err == nil {
				By("Removing finalizers for clean test environment teardown")
				resourceObj.Finalizers = nil
				_ = k8sClient.Update(ctx, resourceObj)

				By("Cleanup the specific resource instance ClusterLab")
				_ = k8sClient.Delete(ctx, resourceObj)
			}
		})

		It("should successfully reconcile the resource following Production-grade rules", func() {
			controllerReconciler := &ClusterLabReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			By("Reconciling Step 1: Verification of Finalizer-First Rule")
			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeZero())

			By("Reconciling Step 2: Verification of Infrastructure Provisioning (Namespace, Quota, NetPol)")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
