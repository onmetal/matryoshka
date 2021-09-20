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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/onmetal/controller-utils/clientutils"
	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/apiserver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("APIServerController", func() {
	const (
		fieldOwner              = "test"
		apiServerSampleFilename = "../../config/samples/matryoshka_v1alpha1_apiserver.yaml"
		tokenSecretName         = "apiserver-token-sample"
		certAndKeySecretName    = "apiserver-cert-and-key"
	)
	ctx := context.Background()
	ns := SetupTest(ctx)

	It("should create a healthy api server", func() {
		By("applying the sample file")
		_, err := clientutils.PatchMultipleFromFile(ctx, client.NewNamespacedClient(k8sClient, ns.Name), apiServerSampleFilename, clientutils.ApplyAll, client.FieldOwner(fieldOwner))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for a deployment to be created")
		deployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "apiserver-sample"}, deployment)
		}, 3*time.Second).Should(Succeed())

		By("inspecting the deployment template")
		template := deployment.Spec.Template
		Expect(template.Labels).NotTo(BeEmpty())
		Expect(deployment.Spec.Selector.MatchLabels).To(Equal(template.Labels))

		By("inspecting the volumes")
		Expect(template.Spec.Volumes).To(ConsistOf(
			corev1.Volume{
				Name: apiserver.ServiceAccountVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: apiserver.TLSVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: apiserver.TokenVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tokenSecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		))

		By("inspecting the template containers")
		Expect(template.Spec.Containers).To(HaveLen(1))
		container := template.Spec.Containers[0]
		Expect(container.Command).To(Equal([]string{
			"/usr/local/bin/kube-apiserver",
			"--enable-admission-plugins=NamespaceLifecycle,NodeRestriction,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota",
			"--allow-privileged=false",
			"--authorization-mode=Node,RBAC",
			"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
			"--event-ttl=1h",
			"--profiling=false",
			"--secure-port=443",
			"--bind-address=0.0.0.0",
			"--service-cluster-ip-range=100.64.0.0/24",
			"--etcd-servers=http://apiserver-etcd-sample:2379",
			"--enable-bootstrap-token-auth=false",
			"--anonymous-auth=true",
			"--service-account-issuer=https://apiserver-sample:443",
			fmt.Sprintf("--service-account-key-file=%s/tls.key", apiserver.ServiceAccountVolumePath),
			fmt.Sprintf("--service-account-signing-key-file=%s/tls.key", apiserver.ServiceAccountVolumePath),
			fmt.Sprintf("--tls-cert-file=%s/tls.crt", apiserver.TLSVolumePath),
			fmt.Sprintf("--tls-private-key-file=%s/tls.key", apiserver.TLSVolumePath),
			fmt.Sprintf("--token-auth-file=%s/%s", apiserver.TokenVolumePath, matryoshkav1alpha1.DefaultAPIServerTokenAuthenticationKey),
		}))
		Expect(container.Resources).To(Equal(corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				"cpu":    resource.MustParse("200m"),
				"memory": resource.MustParse("300Mi"),
			},
			Limits: map[corev1.ResourceName]resource.Quantity{
				"cpu":    resource.MustParse("1200m"),
				"memory": resource.MustParse("2000Mi"),
			},
		}))
		Expect(container.VolumeMounts).To(ConsistOf(
			corev1.VolumeMount{
				Name:      apiserver.ServiceAccountVolumeName,
				MountPath: apiserver.ServiceAccountVolumePath,
			},
			corev1.VolumeMount{
				Name:      apiserver.TLSVolumeName,
				MountPath: apiserver.TLSVolumePath,
			},
			corev1.VolumeMount{
				Name:      apiserver.TokenVolumeName,
				MountPath: apiserver.TokenVolumePath,
			},
		))
		Expect(container.ReadinessProbe).To(Equal(&corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/readyz",
					Port:   intstr.FromInt(443),
					Scheme: corev1.URISchemeHTTPS,
					HTTPHeaders: []corev1.HTTPHeader{
						{
							Name:  "Authorization",
							Value: "Bearer some-token",
						},
					},
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       30,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}))
		Expect(container.LivenessProbe).To(Equal(&corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/livez",
					Port:   intstr.FromInt(443),
					Scheme: corev1.URISchemeHTTPS,
					HTTPHeaders: []corev1.HTTPHeader{
						{
							Name:  "Authorization",
							Value: "Bearer some-token",
						},
					},
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       30,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}))
	})
})
