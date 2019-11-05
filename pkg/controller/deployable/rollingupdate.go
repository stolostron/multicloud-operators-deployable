// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package deployable

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"

	appv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-deployable/pkg/utils"
	subv1alpha1 "github.com/IBM/multicloud-operators-subscription/pkg/apis/app/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
)

func (r *ReconcileDeployable) rollingUpdate(instance *appv1alpha1.Deployable) error {
	if klog.V(utils.QuiteLogLel) {
		fnName := utils.GetFnName()
		klog.Infof("Entering: %v()", fnName)

		defer klog.Infof("Exiting: %v()", fnName)
	}

	klog.V(5).Info("Rolling Updating ", instance)

	annotations := instance.GetAnnotations()

	if annotations == nil || annotations[appv1alpha1.AnnotationRollingUpdateTarget] == "" {
		klog.V(5).Info("Empty annotation or No rolling update target in annotations", annotations)

		return nil
	}

	// maxunav is the actual updated number in every rolling update
	maxunav, err := strconv.Atoi(annotations[appv1alpha1.AnnotationRollingUpdateMaxUnavailable])
	if err != nil {
		maxunav = appv1alpha1.DefaultRollingUpdateMaxUnavailablePercentage
	}

	maxunav = (len(instance.Status.PropagatedStatus)*maxunav + 99) / 100
	klog.V(5).Info("ongoing rolling update to ", annotations[appv1alpha1.AnnotationRollingUpdateTarget], " with max ", maxunav, " unavaialble clusters")

	targetdpl := &appv1alpha1.Deployable{}
	err = r.Get(context.TODO(),
		types.NamespacedName{
			Name:      annotations[appv1alpha1.AnnotationRollingUpdateTarget],
			Namespace: instance.Namespace,
		}, targetdpl)

	if err != nil {
		klog.Info("Failed to find rolling update target", annotations[appv1alpha1.AnnotationRollingUpdateTarget])

		return nil
	}

	targetSubTplPackageOverrides, err := handleSubscriptionPackageOverrides(instance, targetdpl)

	if err != nil {
		return nil
	}

	packageOverrides := getPackageOverrides(targetSubTplPackageOverrides)

	//it is only triggered in the initial rolling update.
	if !reflect.DeepEqual(instance.Spec.Template, targetdpl.Spec.Template) {
		klog.V(5).Info("Initialize rolling update to ", annotations[appv1alpha1.AnnotationRollingUpdateTarget])

		ov := appv1alpha1.Overrides{}

		// target dpl becomes new instnace template for propagation.
		// So instance Overrides stores all clusters who are not eligible to update now.
		// When the whole rolling update is done, the instance Overrides array should be zero element.
		ov.ClusterOverrides = utils.GenerateOverrides(targetdpl, instance)

		covmap := make(map[string]appv1alpha1.Overrides)

		for n := range instance.Status.PropagatedStatus {
			cov := *(ov.DeepCopy())
			cov.ClusterName = n
			covmap[n] = cov
		}
		// existing overrides are rolling out anyway
		maxunav -= len(instance.Spec.Overrides)

		for _, ov := range targetdpl.Spec.Overrides {
			covmap[ov.ClusterName] = *(ov.DeepCopy())
		}

		maxunav -= len(targetdpl.Spec.Overrides)

		// append target subscription template spec.packageOverrides to cluster overrides
		for _, packageOverride := range packageOverrides {
			for cluster, clusterOv := range covmap {
				clusterOv.ClusterOverrides = append(clusterOv.ClusterOverrides, packageOverride)
				covmap[cluster] = clusterOv
			}
		}

		instance.Spec.Overrides = nil
		for _, ov := range covmap {
			instance.Spec.Overrides = append(instance.Spec.Overrides, *(ov.DeepCopy()))
		}

		targetdpl.Spec.Template.DeepCopyInto(instance.Spec.Template)
	}

	for _, cs := range instance.Status.PropagatedStatus {
		if cs.Phase != appv1alpha1.DeployableDeployed {
			maxunav--
		}
	}

	//append the target subscription template spec.packageOverrides to target cluster override as well
	for _, packageOverride := range packageOverrides {
		for ovindex, clusterOv := range targetdpl.Spec.Overrides {
			clusterOv.ClusterOverrides = append(clusterOv.ClusterOverrides, packageOverride)
			targetdpl.Spec.Overrides[ovindex] = clusterOv
		}
	}

	var targetovs []appv1alpha1.Overrides

	ovmap := make(map[string]*appv1alpha1.Overrides)

	for _, tov := range targetdpl.Spec.Overrides {
		ovmap[tov.ClusterName] = tov.DeepCopy()
	}

	for _, ov := range instance.Spec.Overrides {
		// ensure desired overrides are aligned
		if cov, ok := ovmap[ov.ClusterName]; ok {
			targetovs = append(targetovs, *cov)
		} else if maxunav > 0 {
			// roll 1 more
			maxunav--
		} else {
			// out of quota
			cov = &appv1alpha1.Overrides{}
			ov.DeepCopyInto(cov)
			targetovs = append(targetovs, *cov)
		}
	}

	instance.Spec.Overrides = nil

	for _, cov := range targetovs {
		instance.Spec.Overrides = append(instance.Spec.Overrides, *(cov.DeepCopy()))
	}

	klog.V(5).Info("Rolling update exit with overrides: ", instance.Spec.Overrides)

	return nil
}

func getPackageOverrides(targetSubTplPackageOverrides []*subv1alpha1.Overrides) []appv1alpha1.ClusterOverride {
	packageOverrides := []appv1alpha1.ClusterOverride{}

	for _, targetPkgOv := range targetSubTplPackageOverrides {
		ovmap := make(map[string]interface{})
		ovmap["path"] = "spec.packageOverrides"
		ovmap["value"] = targetPkgOv

		patchb, err := json.Marshal(ovmap)

		if err != nil {
			klog.Info("Error in marshal target target subscription template spec.packageOverride ", ovmap, " with error:", err)
			continue
		}

		clusterOverride := appv1alpha1.ClusterOverride{
			RawExtension: runtime.RawExtension{
				Raw: patchb,
			},
		}

		packageOverrides = append(packageOverrides, clusterOverride)
	}

	return packageOverrides
}

// if the dpl template is subscription containing PackageOverrides slice,
// copy over the instance template PackageOverrides with the target template PackageOverrides
// As a result, the Spec.PackageOverrides in instance dpl and target dpl are cleaned up to avoid array patch diff panic
// Also the target Spec.PackageOverrides is returned which will be appended to all cluster overrides later.
func handleSubscriptionPackageOverrides(instance, targetdpl *appv1alpha1.Deployable) ([]*subv1alpha1.Overrides, error) {
	org := &unstructured.Unstructured{}
	err := json.Unmarshal(instance.Spec.Template.Raw, org)

	if err != nil {
		klog.V(5).Info("Error in unmarshall instance template, err:", err, " |template: ", string(instance.Spec.Template.Raw))
		return nil, nil
	}

	if org.GetKind() != "Subscription" {
		return nil, nil
	}

	targetSubTpl := &subv1alpha1.Subscription{}
	err = json.Unmarshal(targetdpl.Spec.Template.Raw, targetSubTpl)

	if err != nil {
		klog.V(5).Info("Error in unmarshal target template, err:", err, " |template: ", string(targetdpl.Spec.Template.Raw))
		return nil, err
	}

	targetSubTplPackageOverrides := targetSubTpl.Spec.PackageOverrides
	targetSubTpl.Spec.PackageOverrides = nil

	targetdpl.Spec.Template.Raw, err = json.Marshal(targetSubTpl)

	if err != nil {
		klog.V(5).Info("Error in marshal target template, err:", err, " |template: ", string(targetdpl.Spec.Template.Raw))
		return nil, err
	}

	instanceSubTpl := &subv1alpha1.Subscription{}
	err = json.Unmarshal(instance.Spec.Template.Raw, instanceSubTpl)

	if err != nil {
		klog.V(5).Info("Error in unmarshal instance template, err:", err, " |template: ", string(instance.Spec.Template.Raw))
		return nil, err
	}

	instanceSubTpl.Spec.PackageOverrides = nil

	instance.Spec.Template.Raw, err = json.Marshal(instanceSubTpl)

	if err != nil {
		klog.V(5).Info("Error in marshal instance template, err:", err, " |template: ", string(instance.Spec.Template.Raw))
		return nil, err
	}

	return targetSubTplPackageOverrides, nil
}

func (r *ReconcileDeployable) validateOverridesForRollingUpdate(instance *appv1alpha1.Deployable) {
	if klog.V(utils.QuiteLogLel) {
		fnName := utils.GetFnName()
		klog.Infof("Entering: %v()", fnName)

		defer klog.Infof("Exiting: %v()", fnName)
	}

	klog.V(5).Info("Rolling update validation started with overrides: ", instance.Spec.Overrides, "and status ", instance.Status.PropagatedStatus)

	var allov []appv1alpha1.Overrides

	for _, ov := range instance.Spec.Overrides {
		klog.V(5).Info("validating overrides: ", ov)

		if _, ok := instance.Status.PropagatedStatus[ov.ClusterName]; ok {
			allov = append(allov, *(ov.DeepCopy()))
		}
	}

	instance.Spec.Overrides = allov

	klog.V(5).Info("Rolling update validated overrides: ", instance.Spec.Overrides)
}
