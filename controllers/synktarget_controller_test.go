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
	"time"

	synkv1alpha1 "github.com/VerwaerdeWim/Synk/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	timeout  = time.Second * 10
	interval = time.Millisecond * 250
)

func createConfigMap(client client.Client, name string, ns string, data map[string]string) *corev1.ConfigMap {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Data: data,
	}
	By("Configmap creation")
	Expect(client.Create(ctx, configMap)).Should(Succeed())
	createdConfigMap := &corev1.ConfigMap{}
	Eventually(func() bool {
		err := client.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, createdConfigMap)
		return err == nil
	}, timeout, interval).Should(BeTrue())
	return createdConfigMap
}

func createSynkSource(client client.Client, name string, ns string, resources []synkv1alpha1.Resource) *synkv1alpha1.SynkSource {
	synkSource := &synkv1alpha1.SynkSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: synkv1alpha1.SynkSourceSpec{
			Resources: resources,
		},
	}
	By("Synksource creation")
	Expect(client.Create(ctx, synkSource)).Should(Succeed())

	createdSynkSource := &synkv1alpha1.SynkSource{}
	Eventually(func() *synkv1alpha1.Connection {
		_ = client.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, createdSynkSource)
		return createdSynkSource.Spec.Connection
	}, timeout, interval).ShouldNot(BeNil())
	return createdSynkSource
}

func createSynkTarget(client client.Client, name string, ns string, synkSource *synkv1alpha1.SynkSource) *synkv1alpha1.SynkTarget {
	By("Copying the synksource to synktarget")
	synkTarget := &synkv1alpha1.SynkTarget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
	synkTarget.Spec.Connection = synkSource.Spec.Connection
	synkTarget.Spec.Resources = synkSource.Spec.Resources
	Expect(client.Create(ctx, synkTarget)).Should(Succeed())

	createdSynkTarget := &synkv1alpha1.SynkTarget{}
	Eventually(func() bool {
		err := k8sClient1.Get(ctx, types.NamespacedName{Name: name, Namespace: ns}, createdSynkTarget)
		return err == nil
	}, timeout, interval).Should(BeTrue())
	return createdSynkTarget
}

func deleteSynkSource(client client.Client, synkSource *synkv1alpha1.SynkSource) {
	By("Synksource deletion")
	Expect(client.Delete(ctx, synkSource)).Should(Succeed())

	By("Checking if the synksource is deleted")
	Eventually(func() bool {
		err := client.Get(ctx, types.NamespacedName{Name: synkSource.Name, Namespace: synkSource.Namespace}, synkSource)
		return err == nil
	}, timeout, interval).Should(BeFalse())
}

var _ = Describe("SynkTarget controller", func() {
	Context("SynkTarget actions", func() {
		Context("SynkTarget create", func() {
			It("Create synksource first", func() {
				By("Checking if sync works")
				// createdConfigMap := createConfigMap(k8sClient2, "test-target-configmap", "test", map[string]string{"hello": "world"})

				createdSynkSource := createSynkSource(k8sClient2, "test-target", "test", []synkv1alpha1.Resource{
					{
						Group:        "",
						Version:      "v1",
						ResourceType: "configmaps",
						Namespace:    "test",
						Names: []string{
							"test-target-configmap",
						},
					},
				})

				// createdSynkTarget := createSynkTarget(k8sClient1, "test-target", "test", createdSynkSource)

				// By("Checking if the configmap is duplicated")
				// createdConfigMap2 := &corev1.ConfigMap{}
				// Eventually(func() bool {
				// 	err := k8sClient1.Get(ctx, types.NamespacedName{Name: "test-target-configmap", Namespace: "test"}, createdConfigMap2)
				// 	return err == nil
				// }, timeout, interval).Should(BeTrue())

				// By("Checking if the configmap contains the right data")
				// Expect(createdConfigMap2.Data).Should(Equal(createdConfigMap.Data))

				// By("Updating the configmap")
				// createdConfigMap.Data["test"] = "test"
				// Expect(k8sClient2.Update(ctx, createdConfigMap)).Should(Succeed())

				// Eventually(func() map[string]string {
				// 	_ = k8sClient1.Get(ctx, types.NamespacedName{Name: "test-target-configmap", Namespace: "test"}, createdConfigMap2)
				// 	return createdConfigMap2.Data
				// }, timeout, interval).Should(Equal(createdConfigMap.Data))

				// By("Creating another configmap")
				// createdConfigMap3 := createConfigMap(k8sClient2, "test-target-configmap2", "test", map[string]string{"hello": "world2"})

				// By("Updating the synksource")

				// createdSynkSource.Spec.Resources[0].Names = append(createdSynkSource.Spec.Resources[0].Names, createdConfigMap3.Name)

				// Expect(k8sClient2.Update(ctx, createdSynkSource)).Should(Succeed())

				// By("Updating the synktarget")
				// createdSynkTarget.Spec.Resources[0].Names = createdSynkSource.Spec.Resources[0].Names
				// Expect(k8sClient1.Update(ctx, createdSynkTarget)).Should(Succeed())

				// By("Checking if the configmap is duplicated")
				// createdConfigMap4 := &corev1.ConfigMap{}
				// Eventually(func() bool {
				// 	err := k8sClient1.Get(ctx, types.NamespacedName{Name: "test-target-configmap2", Namespace: "test"}, createdConfigMap4)
				// 	return err == nil
				// }, timeout, interval).Should(BeTrue())

				By("testing alle resources of a certain type")
				createdSynkSource2 := createSynkSource(k8sClient2, "test-target2", "test", []synkv1alpha1.Resource{
					{
						Group:        "",
						Version:      "v1",
						ResourceType: "configmaps",
						Namespace:    "test2",
					},
				})

				// createdConfigMap5 := createConfigMap(k8sClient2, "test-target-configmap", "test2", map[string]string{"hello": "world"})
				// createdConfigMap6 := createConfigMap(k8sClient2, "test-target-configmap2", "test2", map[string]string{"hello": "world2"})

				// _ = createSynkTarget(k8sClient1, "test-target2", "test", createdSynkSource2)
				// By("Checking if the configmap is duplicated")
				// createdConfigMap7 := &corev1.ConfigMap{}
				// Eventually(func() bool {
				// 	err := k8sClient1.Get(ctx, types.NamespacedName{Name: createdConfigMap5.Name, Namespace: "test2"}, createdConfigMap7)
				// 	return err == nil
				// }, timeout, interval).Should(BeTrue())
				// createdConfigMap8 := &corev1.ConfigMap{}
				// Eventually(func() bool {
				// 	err := k8sClient1.Get(ctx, types.NamespacedName{Name: createdConfigMap6.Name, Namespace: "test2"}, createdConfigMap8)
				// 	return err == nil
				// }, timeout, interval).Should(BeTrue())

				deleteSynkSource(k8sClient2, createdSynkSource)
				deleteSynkSource(k8sClient2, createdSynkSource2)
			})
		})
	})
})
