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
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	synkv1alpha1 "github.com/VerwaerdeWim/Synk/api/v1alpha1"
	"github.com/VerwaerdeWim/Synk/controllers"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient1 client.Client
	k8sClient2 client.Client
	testEnv1   *envtest.Environment
	testEnv2   *envtest.Environment
	ctx        context.Context
	cancel     context.CancelFunc
	scheme     = runtime.NewScheme()
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")

	var err error

	config1, err := clientcmd.BuildConfigFromFlags("", "../kubeconfig1.out")
	Expect(err).NotTo(HaveOccurred())
	Expect(config1).NotTo(BeNil())

	config2, err := clientcmd.BuildConfigFromFlags("", "../kubeconfig2.out")
	Expect(err).NotTo(HaveOccurred())
	Expect(config2).NotTo(BeNil())

	useCluster := true
	testEnv1 = &envtest.Environment{
		UseExistingCluster:    &useCluster,
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,

		CRDInstallOptions: envtest.CRDInstallOptions{
			CleanUpAfterUse: false,
		},
		Config: config1,
	}

	testEnv2 = &envtest.Environment{
		UseExistingCluster:    &useCluster,
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,

		CRDInstallOptions: envtest.CRDInstallOptions{
			CleanUpAfterUse: false,
		},
		Config: config2,
	}

	cfg1, err := testEnv1.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg1).NotTo(BeNil())

	cfg2, err := testEnv2.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg2).NotTo(BeNil())

	err = synkv1alpha1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = clientgoscheme.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient1, err = client.New(cfg1, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient1).NotTo(BeNil())

	k8sClient2, err = client.New(cfg2, client.Options{Scheme: scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient2).NotTo(BeNil())

	k8sManager1, err := ctrl.NewManager(cfg1, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	k8sManager2, err := ctrl.NewManager(cfg2, ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: "0",
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.SynkSourceReconciler{
		Client: k8sManager1.GetClient(),
		Scheme: k8sManager1.GetScheme(),
		Config: k8sManager1.GetConfig(),
	}).SetupWithManager(k8sManager1)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.SynkTargetReconciler{
		Client: k8sManager1.GetClient(),
		Scheme: k8sManager1.GetScheme(),
		Config: k8sManager1.GetConfig(),
	}).SetupWithManager(k8sManager1)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.SynkSourceReconciler{
		Client: k8sManager2.GetClient(),
		Scheme: k8sManager2.GetScheme(),
		Config: k8sManager2.GetConfig(),
	}).SetupWithManager(k8sManager2)
	Expect(err).ToNot(HaveOccurred())

	err = (&controllers.SynkTargetReconciler{
		Client: k8sManager2.GetClient(),
		Scheme: k8sManager2.GetScheme(),
		Config: k8sManager2.GetConfig(),
	}).SetupWithManager(k8sManager2)
	Expect(err).ToNot(HaveOccurred())

	By("Creating test ns in cluster1")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	Expect(k8sClient1.Create(ctx, ns)).Should(Succeed())

	By("Creating test ns in cluster2")
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	Expect(k8sClient2.Create(ctx, ns)).Should(Succeed())

	By("Creating test2 ns in cluster1")
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test2",
		},
	}
	Expect(k8sClient1.Create(ctx, ns)).Should(Succeed())

	By("Creating test3 ns in cluster1")
	ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test3",
		},
	}
	Expect(k8sClient1.Create(ctx, ns)).Should(Succeed())

	go func() {
		defer GinkgoRecover()
		err = k8sManager1.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager1")
	}()
	go func() {
		defer GinkgoRecover()
		err = k8sManager2.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager2")
	}()
}, 60)

var _ = AfterSuite(func() {
	ns := &corev1.Namespace{}

	By("Deleting the namespace in cluster1")
	Expect(k8sClient1.Get(ctx, types.NamespacedName{Name: "test"}, ns)).Should(Succeed())
	Expect(k8sClient1.Delete(ctx, ns)).Should(Succeed())

	By("Deleting the namespace in cluster2")
	Expect(k8sClient2.Get(ctx, types.NamespacedName{Name: "test"}, ns)).Should(Succeed())
	Expect(k8sClient2.Delete(ctx, ns)).Should(Succeed())

	By("Deleting the namespace in cluster1")
	Expect(k8sClient1.Get(ctx, types.NamespacedName{Name: "test2"}, ns)).Should(Succeed())
	Expect(k8sClient1.Delete(ctx, ns)).Should(Succeed())

	By("Deleting the namespace in cluster1")
	Expect(k8sClient1.Get(ctx, types.NamespacedName{Name: "test3"}, ns)).Should(Succeed())
	Expect(k8sClient1.Delete(ctx, ns)).Should(Succeed())

	cancel()
	By("tearing down the test environment")
	err := testEnv1.Stop()
	Expect(err).NotTo(HaveOccurred())
	err = testEnv2.Stop()
	Expect(err).NotTo(HaveOccurred())
})
