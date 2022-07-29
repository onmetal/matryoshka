// Copyright 2021 OnMetal authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matryoshka

import (
	"context"

	"github.com/onmetal/controller-utils/clientutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kubeconfigContent = `apiVersion: v1
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: http://localhost:8080
  name: default
contexts:
- context:
    cluster: default
    user: default
  name: default
current-context: default
kind: Config
preferences: {}
users:
- name: default
  user:
    password: mysupersafepassword
    username: default
`

var _ = Describe("KubeconfigController", func() {
	const (
		fieldOwner               = "test"
		kubeconfigSampleFilename = "../../config/samples/matryoshka_v1alpha1_kubeconfig.yaml"
	)
	ctx := context.Background()
	ns := SetupTest(ctx)

	It("should create a healthy api server", func() {
		By("applying the sample file")
		_, err := clientutils.PatchMultipleFromFile(ctx, client.NewNamespacedClient(k8sClient, ns.Name), kubeconfigSampleFilename, clientutils.ApplyAll, client.FieldOwner(fieldOwner))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for a secret to be created")
		secret := &corev1.Secret{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "my-kubeconfig"}, secret)
		}).Should(Succeed())

		By("inspecting the secret")
		Expect(secret.Data).To(Equal(map[string][]byte{
			matryoshkav1alpha1.DefaultKubeconfigKey: []byte(kubeconfigContent),
		}))
	})
})
