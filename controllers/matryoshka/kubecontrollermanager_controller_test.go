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

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"

	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubecontrollermanager"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeControllerManagerController", func() {
	const (
		certAndKeySecretName = "apiserver-cert-and-key"
	)
	ctx := context.Background()
	ns := SetupTest(ctx)

	It("should create a healthy kube controller manager", func() {
		By("applying the sample file")
		_, err := ApplyFile(ctx, k8sClient, ns.Name, KubeAPIServerSampleFilename)
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyFile(ctx, k8sClient, ns.Name, KubeControllerManagerSampleFilename)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for a deployment to be created")
		deployment := &appsv1.Deployment{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "kubecontrollermanager-sample"}, deployment)
		}, 3*time.Second).Should(Succeed())

		By("inspecting the deployment")
		Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"matryoshka.onmetal.de/app": "kubecontrollermanager-kubecontrollermanager-sample",
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
				Name: kubecontrollermanager.KubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "kcm-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubecontrollermanager.ServiceAccountPrivateKeyVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubecontrollermanager.ServiceAccountRootCertificateVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  certAndKeySecretName,
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubecontrollermanager.AuthorizationKubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "kcm-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubecontrollermanager.AuthenticationKubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "kcm-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		))

		By("inspecting the template containers")
		Expect(template.Spec.Containers).To(HaveLen(1))
		container := template.Spec.Containers[0]
		Expect(container.Command).NotTo(BeEmpty())
		Expect(container.Command[0]).To(Equal("/usr/local/bin/kube-controller-manager"))
		flags := container.Command[1:]
		Expect(flags).To(ConsistOf(
			"--bind-address=0.0.0.0",
			"--leader-elect=true",
			"--v=2",
			"--cluster-name=my-cluster",
			"--controllers=*,bootstrapsigner,tokencleaner",
			"--use-service-account-credentials=true",
			fmt.Sprintf("--kubeconfig=%s/%s", kubecontrollermanager.KubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeControllerManagerGenericConfigurationKubeconfigKey),
			fmt.Sprintf("--service-account-private-key-file=%s/%s", kubecontrollermanager.ServiceAccountPrivateKeyVolumePath, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountControllerConfigurationPrivateKeySecretKey),
			fmt.Sprintf("--root-ca-file=%s/%s", kubecontrollermanager.ServiceAccountRootCertificateVolumePath, matryoshkav1alpha1.DefaultKubeControllerManagerServiceAccountControllerConfigurationRootCertificateSecretKey),
			fmt.Sprintf("--authorization-kubeconfig=%s/%s", kubecontrollermanager.AuthorizationKubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeControllerManagerAuthorizationKubeconfigSecretKey),
			fmt.Sprintf("--authentication-kubeconfig=%s/%s", kubecontrollermanager.AuthenticationKubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeControllerManagerAuthenticationKubeconfigSecretKey),
			"--authentication-skip-lookup=true",
		))
		Expect(container.VolumeMounts).To(ConsistOf(
			corev1.VolumeMount{
				Name:      kubecontrollermanager.KubeconfigVolumeName,
				MountPath: kubecontrollermanager.KubeconfigVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubecontrollermanager.ServiceAccountPrivateKeyVolumeName,
				MountPath: kubecontrollermanager.ServiceAccountPrivateKeyVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubecontrollermanager.ServiceAccountRootCertificateVolumeName,
				MountPath: kubecontrollermanager.ServiceAccountRootCertificateVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubecontrollermanager.AuthorizationKubeconfigVolumeName,
				MountPath: kubecontrollermanager.AuthorizationKubeconfigVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubecontrollermanager.AuthenticationKubeconfigVolumeName,
				MountPath: kubecontrollermanager.AuthenticationKubeconfigVolumePath,
			},
		))
		Expect(container.LivenessProbe).To(Equal(&corev1.Probe{
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt(10257),
					Scheme: corev1.URISchemeHTTPS,
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			PeriodSeconds:       10,
			SuccessThreshold:    1,
			FailureThreshold:    2,
		}))
	})
})
