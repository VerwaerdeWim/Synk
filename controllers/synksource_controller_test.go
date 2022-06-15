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

package controllers

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
		TestSynkSourceNamespace = "test"
		timeout                 = time.Second * 10
		interval                = time.Millisecond * 250
	)

	Context("SynkSource creation", func() {
		ctx := context.Background()
		It("Should allow watch rights to all listed sources", func() {
			By("Creating a SynkSource resource")
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
			Expect(k8sClient.Create(ctx, synkSource)).Should(Succeed())

			createdSynkSource := &synkv1alpha1.SynkSource{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, createdSynkSource)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Creating a Service account for the SynkSource")
			sa := &corev1.ServiceAccount{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, sa)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Creating a Secret for the service account")
			secret := &corev1.Secret{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, secret)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(secret.Type).Should(Equal(corev1.SecretTypeServiceAccountToken))
			Expect(secret.ObjectMeta.Annotations["kubernetes.io/service-account.name"]).Should(Equal(TestSynkSourceName))
			Expect(len(secret.ObjectMeta.Annotations)).Should(Equal(2))

			By("Creating a Role for the resource")
			role := &rbacv1.Role{}
			Eventually(func() bool {
				roleName := TestSynkSourceName
				err := k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: TestConfigMapNamespace}, role)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(role.Rules[0].Verbs[0]).Should(Equal("watch"))
			Expect(len(role.Rules)).Should(Equal(2))
			Expect(len(role.Rules[0].ResourceNames)).Should(Equal(3))
			Expect(role.Rules[0].ResourceNames).Should(ConsistOf(TestConfigMapName, TestConfigMapName2, TestConfigMapName3))

			role2 := &rbacv1.Role{}
			Eventually(func() bool {
				roleName := TestSynkSourceName
				err := k8sClient.Get(ctx, types.NamespacedName{Name: roleName, Namespace: TestConfigMapNamespace2}, role2)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(role2.Rules[0].Verbs[0]).Should(Equal("watch"))
			Expect(len(role2.Rules)).Should(Equal(1))
			Expect(role2.Rules[0].ResourceNames).Should(BeEmpty())

			By("Creating a RoleBinding for each Role")
			roleBinding := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				roleBindingName := TestSynkSourceName
				err := k8sClient.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: TestConfigMapNamespace}, roleBinding)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(roleBinding.Subjects[0].Kind).Should(Equal("ServiceAccount"))
			Expect(roleBinding.Subjects[0].Name).Should(Equal(TestSynkSourceName))
			Expect(roleBinding.RoleRef.Kind).Should(Equal("Role"))
			Expect(roleBinding.RoleRef.Name).Should(Equal(TestSynkSourceName))
			Expect(roleBinding.RoleRef.APIGroup).Should(Equal("rbac.authorization.k8s.io"))

			roleBinding2 := &rbacv1.RoleBinding{}
			Eventually(func() bool {
				roleBindingName := TestSynkSourceName
				err := k8sClient.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: TestConfigMapNamespace2}, roleBinding2)
				return err == nil
			}, timeout, interval).Should(BeTrue())
			Expect(roleBinding2.Subjects[0].Kind).Should(Equal("ServiceAccount"))
			Expect(roleBinding2.Subjects[0].Name).Should(Equal(TestSynkSourceName))
			Expect(roleBinding2.RoleRef.Kind).Should(Equal("Role"))
			Expect(roleBinding2.RoleRef.Name).Should(Equal(TestSynkSourceName))
			Expect(roleBinding2.RoleRef.APIGroup).Should(Equal("rbac.authorization.k8s.io"))

			By("Deleting SynkSource must delete rights")
			Expect(k8sClient.Delete(ctx, createdSynkSource)).Should(Succeed())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, createdSynkSource)
				return err == nil
			}, timeout, interval).Should(BeFalse())

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

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, secret)
				return err == nil
			}, timeout, interval).Should(BeFalse())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: TestSynkSourceName, Namespace: TestSynkSourceNamespace}, sa)
				return err == nil
			}, timeout, interval).Should(BeFalse())
		})
	})
})
