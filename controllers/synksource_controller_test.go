/*
Copyright 2022 VerwaerdeWim.

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
	"time"

	synkv1alpha1 "github.com/VerwaerdeWim/Synk/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("SynkSource controller", func() {
	const (
		TestConfigMapName       = "test-configmap"
		TestConfigMapName2      = "test-configmap2"
		TestConfigMapName3      = "test-configmap3"
		TestConfigMapName4      = "test-configmap4"
		TestSynkSourceName      = "test-synksource"
		TestSecretName          = "test-secret"
		TestSecretNamespace     = "test"
		TestConfigMapNamespace  = "test"
		TestConfigMapNamespace2 = "test2"
		TestSecretNamespace2    = "test3"
		TestSynkSourceNamespace = "test"
		timeout                 = time.Second * 10
		interval                = time.Millisecond * 250
	)

	Context("SynkSource actions", func() {
		ctx := context.Background()
		var (
			createdSynkSource *synkv1alpha1.SynkSource = &synkv1alpha1.SynkSource{}
			role              *rbacv1.Role             = &rbacv1.Role{}
			role2             *rbacv1.Role             = &rbacv1.Role{}
			roleBinding       *rbacv1.RoleBinding      = &rbacv1.RoleBinding{}
			roleBinding2      *rbacv1.RoleBinding      = &rbacv1.RoleBinding{}
			secret            *corev1.Secret           = &corev1.Secret{}
			sa                *corev1.ServiceAccount   = &corev1.ServiceAccount{}
		)

		Context("SynkSource creation", func() {
			It("Should create a SynkSource resource", func() {
				synkSource := &synkv1alpha1.SynkSource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      TestSynkSourceName,
						Namespace: TestSynkSourceNamespace,
					},
					Spec: synkv1alpha1.SynkSourceSpec{
						Resources: []synkv1alpha1.Resource{
							{
								Group:        "",
								Version:      "v1",
								ResourceType: "configmaps",
								Namespace:    TestConfigMapNamespace,
								Names: []string{
									TestConfigMapName,
									TestConfigMapName2,
								},
							},
							{
								Group:        "",
								Version:      "v1",
								ResourceType: "configmaps",
								Namespace:    TestConfigMapNamespace,
								Names: []string{
									TestConfigMapName3,
								},
							},
							{
								Group:        "",
								Version:      "v1",
								ResourceType: "secrets",
								Namespace:    TestSecretNamespace,
								Names: []string{
									TestSecretName,
								},
							},
							{
								Group:        "",
								Version:      "v1",
								ResourceType: "configmaps",
								Namespace:    TestConfigMapNamespace2,
							},
						},
					},
				}
				By("Creating the SynkSource")
				Expect(k8sClient.Create(ctx, synkSource)).Should(Succeed())

				By("Checking if the created SynkSource exists")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, createdSynkSource)
					return err == nil
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a Service account", func() {
				By("Checking if the service account exists")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, sa)
					return err == nil
				}, timeout, interval).Should(BeTrue())
			})

			It("Should create a Secret for the service account", func() {
				By("Checking if the secret exists")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, secret)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				By("Checking if the type is correct")
				Expect(secret.Type).Should(Equal(corev1.SecretTypeServiceAccountToken))

				By("Checking if the service account is correctly linked to the secret")
				Expect(secret.ObjectMeta.Annotations["kubernetes.io/service-account.name"]).Should(Equal(TestSynkSourceName))
				Expect(len(secret.ObjectMeta.Annotations)).Should(Equal(2))

				By("Checking if the secret contains the generated data")
				Expect(len(secret.Data)).Should(Equal(3))
			})

			It("Should create Roles", func() {
				By("Checking if the roles exist")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestConfigMapNamespace}, role)
					return err == nil
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestConfigMapNamespace2}, role2)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				By("Checking if the amount of rules is correct")
				Expect(len(role.Rules)).Should(Equal(2))

				Expect(len(role2.Rules)).Should(Equal(1))

				By("Checking if each rule has only one resourcetype")
				Expect(len(role.Rules[0].Resources)).Should(Equal(1))
				Expect(len(role.Rules[1].Resources)).Should(Equal(1))

				Expect(len(role2.Rules[0].Resources)).Should(Equal(1))

				By("Checking if the only verb rule is watch")
				Expect(len(role.Rules[0].Verbs)).Should(Equal(1))
				Expect(len(role.Rules[1].Verbs)).Should(Equal(1))
				Expect(role.Rules[0].Verbs[0]).Should(Equal("watch"))
				Expect(role.Rules[1].Verbs[0]).Should(Equal("watch"))

				Expect(len(role2.Rules[0].Verbs)).Should(Equal(1))
				Expect(role2.Rules[0].Verbs[0]).Should(Equal("watch"))

				By("Checking if the amount of resourcenames is correct")
				Expect(len(role.Rules[0].ResourceNames)).Should(Equal(3))
				Expect(len(role.Rules[1].ResourceNames)).Should(Equal(1))

				Expect(role2.Rules[0].ResourceNames).Should(BeEmpty())

				By("Checking if the resourcenames are correct")
				Expect(role.Rules[0].ResourceNames).Should(ConsistOf(TestConfigMapName, TestConfigMapName2, TestConfigMapName3))
				Expect(role.Rules[1].ResourceNames).Should(ConsistOf(TestSecretName))
			})

			It("Should create Rolebindings", func() {
				By("Checking if the rolebindings exist")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestConfigMapNamespace}, roleBinding)
					return err == nil
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestConfigMapNamespace2}, roleBinding2)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				By("Checking if there is only 1 subject")
				Expect(len(roleBinding.Subjects)).Should(Equal(1))

				Expect(len(roleBinding2.Subjects)).Should(Equal(1))

				By("Checking if it is the right subject")
				Expect(roleBinding.Subjects[0].Kind).Should(Equal("ServiceAccount"))
				Expect(roleBinding.Subjects[0].Name).Should(Equal(TestSynkSourceName))

				Expect(roleBinding2.Subjects[0].Kind).Should(Equal("ServiceAccount"))
				Expect(roleBinding2.Subjects[0].Name).Should(Equal(TestSynkSourceName))

				By("Checking if it has the correct roleref")
				Expect(roleBinding.RoleRef.Kind).Should(Equal("Role"))
				Expect(roleBinding.RoleRef.Name).Should(Equal(TestSynkSourceName))
				Expect(roleBinding.RoleRef.APIGroup).Should(Equal("rbac.authorization.k8s.io"))

				Expect(roleBinding2.RoleRef.Kind).Should(Equal("Role"))
				Expect(roleBinding2.RoleRef.Name).Should(Equal(TestSynkSourceName))
				Expect(roleBinding2.RoleRef.APIGroup).Should(Equal("rbac.authorization.k8s.io"))
			})

			It("Should add connection parameters to the SynkSource", func() {
				By("Checking if the connection is not nil")
				Eventually(func() *synkv1alpha1.Connection {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, createdSynkSource)
					return createdSynkSource.Spec.Connection
				}, timeout, interval).ShouldNot(BeNil())
			})
		})

		Context("SynkSource update", func() {
			It("Should update the synksource", func() {
				By("Updating Synksource")
				createdSynkSource.Spec.Resources[0].Names = []string{TestConfigMapName}
				createdSynkSource.Spec.Resources = append(createdSynkSource.Spec.Resources, []synkv1alpha1.Resource{{
					Group:        "",
					Version:      "v1",
					ResourceType: "secrets",
					Namespace:    TestSecretNamespace2,
					Names: []string{
						TestSecretName,
					},
				}}...)
				Expect(k8sClient.Update(ctx, createdSynkSource)).Should(Succeed())
			})

			var role3 *rbacv1.Role = &rbacv1.Role{}
			var roleBinding3 *rbacv1.RoleBinding = &rbacv1.RoleBinding{}
			It("Should update the roles", func() {
				By("Checking if the amount of resource names has changed correctly")
				Eventually(func() int {
					_ = k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestConfigMapNamespace}, role)
					return len(role.Rules[0].ResourceNames)
				}, timeout, interval).Should(Equal(2))

				By("Checking if a role and rolebinding are created")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSecretNamespace2}, role3)
					return err == nil
				}, timeout, interval).Should(BeTrue())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSecretNamespace2}, roleBinding3)
					return err == nil
				}, timeout, interval).Should(BeTrue())

				By("Checking if the amount of rules are correct")
				Expect(len(role.Rules)).Should(Equal(2))
				Expect(len(role3.Rules)).Should(Equal(1))
			})

			It("Should update again to remove role3 and rolebinding3", func() {
				By("Updating the synksource")
				createdSynkSource.Spec.Resources = createdSynkSource.Spec.Resources[:len(createdSynkSource.Spec.Resources)-1]
				Expect(k8sClient.Update(ctx, createdSynkSource)).Should(Succeed())

				By("Checking if the role and rolebinding not exist anymore")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSecretNamespace2}, role3)
					return err == nil
				}, timeout, interval).Should(BeFalse())
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSecretNamespace2}, roleBinding3)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})
		})

		Context("SynkSource deletion", func() {
			It("Should delete the SynkSource resource", func() {
				By("Deleting the SynkSource")
				Expect(k8sClient.Delete(ctx, createdSynkSource)).Should(Succeed())

				By("Checking if the SynkSource is deleted")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, createdSynkSource)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})

			It("Should delete the rolebindings", func() {
				By("Checking if the rolebindings not exist anymore")
				Eventually(func() bool {
					roleBindingName := TestSynkSourceName
					err := k8sClient.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: TestConfigMapNamespace}, roleBinding)
					return err == nil
				}, timeout, interval).Should(BeFalse())

				Eventually(func() bool {
					roleBindingName := TestSynkSourceName
					err := k8sClient.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: TestConfigMapNamespace}, roleBinding2)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})

			It("Should delete the roles", func() {
				By("Checking if the roles not exist anymore")
				Eventually(func() bool {
					roleName := TestSynkSourceName
					err := k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: TestConfigMapNamespace}, role)
					return err == nil
				}, timeout, interval).Should(BeFalse())

				Eventually(func() bool {
					roleName := TestSynkSourceName
					err := k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: TestConfigMapNamespace2}, role2)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})

			It("Should delete the secret", func() {
				By("Checking if the secret not exist anymore")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, secret)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})

			It("Should delete the service account", func() {
				By("Chekcing if the service account not exists anymore")
				Eventually(func() bool {
					err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, sa)
					return err == nil
				}, timeout, interval).Should(BeFalse())
			})
		})
	})
})
