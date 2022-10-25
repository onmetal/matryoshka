// Copyright 2022 OnMetal authors
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

	matryoshkav1alpha1 "github.com/onmetal/matryoshka/apis/matryoshka/v1alpha1"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/kubescheduler"
	"github.com/onmetal/matryoshka/controllers/matryoshka/internal/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeSchedulerController", func() {
	ctx := context.Background()
	ns := SetupTest(ctx)

	It("should create a healthy kube scheduler", func() {
		By("applying the sample file")
		_, err := ApplyFile(ctx, k8sClient, ns.Name, KubeAPIServerSampleFilename)
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyFile(ctx, k8sClient, ns.Name, KubeControllerManagerSampleFilename)
		Expect(err).NotTo(HaveOccurred())
		_, err = ApplyFile(ctx, k8sClient, ns.Name, KubeSchedulerSampleFileName)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for a deployment to be created")
		deployment := &appsv1.Deployment{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Namespace: ns.Name, Name: "kubescheduler-sample"}, deployment)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
			g.Expect(err).NotTo(HaveOccurred())
		}).Should(Succeed())

		By("inspecting the deployment")
		Expect(deployment.Spec.Selector).To(Equal(&metav1.LabelSelector{
			MatchLabels: map[string]string{
				"matryoshka.onmetal.de/app": "kubescheduler-kubescheduler-sample",
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
				Name: kubescheduler.KubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "ks-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubescheduler.AuthorizationKubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "ks-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
			corev1.Volume{
				Name: kubescheduler.AuthenticationKubeconfigVolumeName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  "ks-kubeconfig",
						DefaultMode: pointer.Int32Ptr(420),
					},
				},
			},
		))

		By("inspecting the template containers")
		Expect(template.Spec.Containers).To(HaveLen(1))
		container := template.Spec.Containers[0]
		Expect(container.Command).NotTo(BeEmpty())
		Expect(container.Command[0]).To(Equal("/usr/local/bin/kube-scheduler"))
		flags := container.Command[1:]
		Expect(flags).To(ConsistOf(
			"--bind-address=0.0.0.0",
			"--leader-elect=true",
			"--v=2",
			fmt.Sprintf("--kubeconfig=%s/%s", kubescheduler.KubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeSchedulerConfigurationKubeconfigKey),
			fmt.Sprintf("--authorization-kubeconfig=%s/%s", kubescheduler.AuthorizationKubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeSchedulerAuthorizationKubeconfigSecretKey),
			fmt.Sprintf("--authentication-kubeconfig=%s/%s", kubescheduler.AuthenticationKubeconfigVolumePath, matryoshkav1alpha1.DefaultKubeSchedulerAuthenticationKubeconfigSecretKey),
			"--authentication-skip-lookup=true",
			"--feature-gates=NodeSwap=false",
		))
		Expect(container.VolumeMounts).To(ConsistOf(
			corev1.VolumeMount{
				Name:      kubescheduler.KubeconfigVolumeName,
				MountPath: kubescheduler.KubeconfigVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubescheduler.AuthorizationKubeconfigVolumeName,
				MountPath: kubescheduler.AuthorizationKubeconfigVolumePath,
			},
			corev1.VolumeMount{
				Name:      kubescheduler.AuthenticationKubeconfigVolumeName,
				MountPath: kubescheduler.AuthenticationKubeconfigVolumePath,
			},
		))
		Expect(container.LivenessProbe).To(Equal(&corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path:   "/healthz",
					Port:   intstr.FromInt(10259),
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
