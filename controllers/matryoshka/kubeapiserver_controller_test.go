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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubeapiserver"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"
)

var _ = Describe("KubeAPIServerController", func() {
	const (
		tokenSecretName      = "apiserver-token-sample"
		certAndKeySecretName = "apiserver-cert-and-key"
	)
	ctx := context.Background()
	ns := SetupTest(ctx)

	It("should create a healthy api server", func() {
		By("applying the sample file")
		_, err := ApplyFile(ctx, k8sClient, ns.Name, KubeAPIServerSampleFilename)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for a deployment to be created")
		deployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "apiserver-sample"}, deployment)
		}, 3*time.Second).Should(Succeed())

		By("inspecting the deployment")
		Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"matryoshka.onmetal.de/app": "kubeapiserver-apiserver-sample",
			},
		}))

		By("inspecting the deployment template")
		template := deployment.Spec.Template
		Expect(template.Labels).NotTo(BeEmpty())
		Expect(template.Labels).To(Equal(utils.MergeStringStringMaps(
			deployment.Spec.Selector.MatchLabels,
			map[string]string{"foo": "bar"},
		)))

		By("inspecting the volumes")
		Expect(template.Spec.Volumes).To(ConsistOf(
			corev1.Volume{
				Name: kubeapiserver.ServiceAccountKeyVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubeapiserver.ServiceAccountSigningKeyVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubeapiserver.TLSVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubeapiserver.TokenVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  tokenSecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		))

		By("inspecting the template containers")
		Expect(template.Spec.Containers).To(HaveLen(2))
		container := template.Spec.Containers[0]
		sidecar := template.Spec.Containers[1]
		Expect(container.Command).NotTo(BeEmpty())
		Expect(container.Command[0]).To(Equal("/usr/local/bin/kube-apiserver"))
		flags := container.Command[1:]
		Expect(flags).To(ConsistOf(
			"--enable-admission-plugins=NamespaceLifecycle,NodeRestriction,LimitRanger,ServiceAccount,DefaultStorageClass,ResourceQuota",
			"--runtime-config=api/alpha=false",
			"--allow-privileged=false",
			"--authorization-mode=Node,RBAC",
			"--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP",
			"--event-ttl=1h",
			"--profiling=false",
			"--secure-port=443",
			"--bind-address=0.0.0.0",
			"--service-cluster-ip-range=100.64.0.0/24",
			"--etcd-servers=http://apiserver-etcd-sample:2379",
			"--enable-bootstrap-token-auth=true",
			"--anonymous-auth=true",
			"--service-account-issuer=https://apiserver-sample:443",
			fmt.Sprintf("--service-account-key-file=%s/tls.key", kubeapiserver.ServiceAccountKeyVolumePath),
			fmt.Sprintf("--service-account-signing-key-file=%s/tls.key", kubeapiserver.ServiceAccountSigningKeyVolumePath),
			fmt.Sprintf("--tls-cert-file=%s/tls.crt", kubeapiserver.TLSVolumePath),
			fmt.Sprintf("--tls-private-key-file=%s/tls.key", kubeapiserver.TLSVolumePath),
			fmt.Sprintf("--token-auth-file=%s/%s", kubeapiserver.TokenVolumePath, matryoshkav1alpha1.DefaultKubeAPIServerAuthenticationTokenSecretKey),
			"--feature-gates=GracefulNodeShutdown=true",
		))
		Expect(container.VolumeMounts).To(ConsistOf(
			corev1.VolumeMount{
				Name:      kubeapiserver.ServiceAccountKeyVolumeName,
				MountPath: kubeapiserver.ServiceAccountKeyVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubeapiserver.ServiceAccountSigningKeyVolumeName,
				MountPath: kubeapiserver.ServiceAccountSigningKeyVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubeapiserver.TLSVolumeName,
				MountPath: kubeapiserver.TLSVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubeapiserver.TokenVolumeName,
				MountPath: kubeapiserver.TokenVolumePath,
			},
		))
		Expect(sidecar.ReadinessProbe).To(Equal(&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c", "wget -nv -t1 --spider https://localhost:443/readyz"},
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       30,
			SuccessThreshold:    1,
			FailureThreshold:    3,
		}))
		Expect(sidecar.LivenessProbe).To(Equal(&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				Exec: &corev1.ExecAction{
					Command: []string{"sh", "-c", "wget -nv -t1 --spider https://localhost:443/livez"},
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
