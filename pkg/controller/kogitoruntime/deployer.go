// Copyright 2020 Red Hat, Inc. and/or its affiliates
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

package kogitoruntime

import (
	"fmt"
	"github.com/RHsyseng/operator-utils/pkg/resource"
	"github.com/RHsyseng/operator-utils/pkg/resource/compare"
	monv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/kiegroup/kogito-cloud-operator/pkg/apis/app/v1alpha1"
	"github.com/kiegroup/kogito-cloud-operator/pkg/client"
	"github.com/kiegroup/kogito-cloud-operator/pkg/framework"
	"github.com/kiegroup/kogito-cloud-operator/pkg/infrastructure"
	"github.com/kiegroup/kogito-cloud-operator/pkg/infrastructure/services"
	"github.com/kiegroup/kogito-cloud-operator/pkg/logger"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"reflect"
	"strings"
)

const (
	kogitoHome = "/home/kogito"

	serviceAccountName = "kogito-service-viewer"

	envVarExternalURL = "KOGITO_SERVICE_URL"

	// protobufConfigMapSuffix Suffix that is appended to Protobuf ConfigMap name
	protobufConfigMapSuffix  = "protobuf-files"
	downwardAPIVolumeName    = "podinfo"
	downwardAPIVolumeMount   = kogitoHome + "/" + downwardAPIVolumeName
	downwardAPIProtoBufCMKey = "protobufcm"

	postHookPersistenceScript = kogitoHome + "/launch/post-hook-persistence.sh"

	envVarNamespace = "NAMESPACE"
)

var (
	downwardAPIDefaultMode = int32(420)

	podStartExecCommand = []string{"/bin/bash", "-c", "if [ -x " + postHookPersistenceScript + " ]; then " + postHookPersistenceScript + "; fi"}
	
	log = logger.GetLogger("kogitoruntime_deployer")
)

func onGetComparators(comparator compare.ResourceComparator) {
	comparator.SetComparator(
		framework.NewComparatorBuilder().
			WithType(reflect.TypeOf(corev1.ConfigMap{})).
			WithCustomComparator(protoBufConfigMapComparator).
			Build())

	comparator.SetComparator(
		framework.NewComparatorBuilder().
			WithType(reflect.TypeOf(monv1.ServiceMonitor{})).
			WithCustomComparator(framework.CreateServiceMonitorComparator()).
			Build())
}

func onObjectsCreate(cli *client.Client, kogitoService v1alpha1.KogitoService) (resources map[reflect.Type][]resource.KubernetesResource, lists []runtime.Object, err error) {
	resources = make(map[reflect.Type][]resource.KubernetesResource)

	resObjectList, resType, res := createProtoBufConfigMap(kogitoService)
	lists = append(lists, resObjectList)
	resources[resType] = []resource.KubernetesResource{res}

	resObjectList, resType, res = createPrometheusServiceMonitor(cli, kogitoService)
	if resObjectList != nil {
		lists = append(lists, resObjectList)
		resources[resType] = []resource.KubernetesResource{res}
	}
	return
}

func createProtoBufConfigMap(kogitoService v1alpha1.KogitoService) (runtime.Object, reflect.Type, resource.KubernetesResource) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: kogitoService.GetNamespace(),
			Name:      getProtoBufConfigMapName(kogitoService.GetName()),
			Labels: map[string]string{
				infrastructure.ConfigMapProtoBufEnabledLabelKey: "true",
				framework.LabelAppKey:                           kogitoService.GetName(),
			},
		},
	}
	return &corev1.ConfigMapList{}, reflect.TypeOf(corev1.ConfigMap{}), configMap
}

func createPrometheusServiceMonitor(cli *client.Client, kogitoService v1alpha1.KogitoService) (runtime.Object, reflect.Type, resource.KubernetesResource) {
	if services.IsPrometheusAvailable(cli) {
		kogitoRuntime := kogitoService.(*v1alpha1.KogitoRuntime)
		if serviceMonitor := services.CreateServiceMonitor(kogitoRuntime); serviceMonitor != nil {
			return &monv1.ServiceMonitorList{}, reflect.TypeOf(monv1.ServiceMonitor{}), serviceMonitor
		}
	}
	return nil, nil, nil
}

func protoBufConfigMapComparator(deployed resource.KubernetesResource, requested resource.KubernetesResource) (equal bool) {
	cmDeployed := deployed.(*corev1.ConfigMap)

	// this update is made by the downward API inside the pod container
	if strings.HasSuffix(cmDeployed.Name, protobufConfigMapSuffix) {
		return true
	}

	return framework.CreateConfigMapComparator()(deployed, requested)
}

// onDeploymentCreate hooks into the infrastructure package to add additional capabilities/properties to the deployment creation
func onDeploymentCreate(cli *client.Client, deployment *v1.Deployment, kogitoService v1alpha1.KogitoService) error {
	kogitoRuntime := kogitoService.(*v1alpha1.KogitoRuntime)
	// NAMESPACE service discovery
	framework.SetEnvVar(envVarNamespace, kogitoService.GetNamespace(), &deployment.Spec.Template.Spec.Containers[0])
	// external URL
	log.Info("The external uri is: " + kogitoService.GetStatus().GetExternalURI())
	if kogitoService.GetStatus().GetExternalURI() != "" {
		log.Info("Setting env variable KOGITO_SERVICE_URL")
		framework.SetEnvVar(envVarExternalURL, kogitoService.GetStatus().GetExternalURI(), &deployment.Spec.Template.Spec.Containers[0])
	}
	// sa
	deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	// istio
	if kogitoRuntime.Spec.EnableIstio {
		framework.AddIstioInjectSidecarAnnotation(&deployment.Spec.Template.ObjectMeta)
	}
	// protobuf
	applyProtoBufConfigurations(deployment, kogitoService)

	if err := infrastructure.InjectDataIndexURLIntoKogitoRuntimeDeployment(cli, kogitoService.GetNamespace(), deployment); err != nil {
		return err
	}

	if err := infrastructure.InjectJobsServiceURLIntoKogitoRuntimeDeployment(cli, kogitoService.GetNamespace(), deployment); err != nil {
		return err
	}

	return nil
}

// getProtoBufConfigMapName gets the name of the protobuf configMap based the given KogitoRuntime instance
func getProtoBufConfigMapName(serviceName string) string {
	return fmt.Sprintf("%s-%s", serviceName, protobufConfigMapSuffix)
}

// applyProtoBufConfigurations configures the deployment to handle protobuf
func applyProtoBufConfigurations(deployment *v1.Deployment, kogitoService v1alpha1.KogitoService) {
	deployment.Spec.Template.Labels[downwardAPIProtoBufCMKey] = getProtoBufConfigMapName(kogitoService.GetName())
	deployment.Spec.Template.Spec.Volumes = append(
		deployment.Spec.Template.Spec.Volumes,
		corev1.Volume{
			Name: downwardAPIVolumeName,
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{Path: "name", FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name", APIVersion: "v1"}},
						{Path: downwardAPIProtoBufCMKey, FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.labels['" + downwardAPIProtoBufCMKey + "']", APIVersion: "v1"}},
					},
					DefaultMode: &downwardAPIDefaultMode,
				},
			},
		},
	)
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		deployment.Spec.Template.Spec.Containers[0].VolumeMounts =
			append(
				deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
				corev1.VolumeMount{
					Name:      downwardAPIVolumeName,
					MountPath: downwardAPIVolumeMount,
				})
		deployment.Spec.Template.Spec.Containers[0].Lifecycle =
			&corev1.Lifecycle{PostStart: &corev1.Handler{Exec: &corev1.ExecAction{Command: podStartExecCommand}}}
	}
}
