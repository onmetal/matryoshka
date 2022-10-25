/*
Copyright 2021.

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

package matryoshka

import (
	"context"
	"testing"
	"time"

	"github.com/onmetal/controller-utils/clientutils"
	"github.com/onmetal/controller-utils/modutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(3 * time.Second)
	SetDefaultEventuallyPollingInterval(250 * time.Millisecond)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			modutils.Dir("github.com/onmetal/matryoshka", "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = matryoshkav1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

func SetupTest(ctx context.Context) *corev1.Namespace {
	ns := &corev1.Namespace{}

	mgrCtx, cancelMgr := context.WithCancel(ctx)

	BeforeEach(func() {
		*ns = corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{GenerateName: "foo"},
		}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed(), "failed to create test namespace")

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred(), "failed to create manager")

		Expect((&KubeAPIServerReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr)).To(Succeed(), "failed to setup kube api server controller")

		Expect((&KubeconfigReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr)).To(Succeed(), "failed to setup kubeconfig controller")

		Expect((&KubeControllerManagerReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr)).To(Succeed(), "failed to setup kube controller manager controller")

		Expect((&KubeSchedulerReconciler{
			Scheme: mgr.GetScheme(),
			Client: mgr.GetClient(),
		}).SetupWithManager(mgr)).To(Succeed(), "failed to setup kube scheduler controller")

		go func() {
			Expect(mgr.Start(mgrCtx)).To(Succeed())
		}()
	})

	AfterEach(func() {
		cancelMgr()
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed(), "failed to delete test namespace")
	})

	return ns
}

func ApplyFile(ctx context.Context, c client.Client, namespace, filename string) ([]unstructured.Unstructured, error) {
	return clientutils.PatchMultipleFromFile(ctx, client.NewNamespacedClient(c, namespace), filename, clientutils.ApplyAll, client.FieldOwner("test"))
}

var (
	SamplesPath                         = modutils.Dir("github.com/onmetal/matryoshka", "config", "samples")
	KubeAPIServerSampleFilename         = SamplesPath + "/matryoshka_v1alpha1_kubeapiserver.yaml"
	KubeControllerManagerSampleFilename = SamplesPath + "/matryoshka_v1alpha1_kubecontrollermanager.yaml"
	KubeSchedulerSampleFileName         = SamplesPath + "/matryoshka_v1alpha1_kubescheduler.yaml"
)
