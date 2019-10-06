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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-deployable/pkg/utils"
	placementv1alpha1 "github.com/IBM/multicloud-operators-placementrule/pkg/apis/app/v1alpha1"
)

// Top priority: placementRef, ignore others
// Next priority: clusterNames, ignore selector
// Bottomline: Use label selector
func (r *ReconcileDeployable) getClustersByPlacement(instance *appv1alpha1.Deployable) ([]types.NamespacedName, error) {
	if klog.V(utils.QuiteLogLel) {
		fnName := utils.GetFnName()
		klog.Infof("Entering: %v()", fnName)
		defer klog.Infof("Exiting: %v()", fnName)
	}
	var clusters []types.NamespacedName
	var err error
	// Top priority: placementRef, ignore others
	// Next priority: clusterNames, ignore selector
	// Bottomline: Use label selector
	if instance.Spec.Placement.PlacementRef != nil {
		clusters, err = r.getClustersFromPlacementRef(instance)
	} else {
		var labelSelector *metav1.LabelSelector

		if instance.Spec.Placement.Clusters != nil {
			namereq := metav1.LabelSelectorRequirement{}
			namereq.Key = "name"
			namereq.Operator = metav1.LabelSelectorOpIn
			for _, cl := range instance.Spec.Placement.Clusters {
				namereq.Values = append(namereq.Values, cl.Name)
			}
			labelSelector = &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{namereq},
			}
		} else {
			labelSelector = instance.Spec.Placement.ClusterSelector
		}
		clSelector, err := utils.ConvertLabels(labelSelector)
		if err != nil {
			return nil, err
		}
		klog.V(10).Info("Using Cluster LabelSelector ", clSelector)
		cllist := &clusterv1alpha1.ClusterList{}
		err = r.List(context.TODO(), &client.ListOptions{LabelSelector: clSelector}, cllist)
		if err != nil && !errors.IsNotFound(err) {
			klog.Error("Listing clusters and found error: ", err)
			return nil, err
		}

		var allcls []*clusterv1alpha1.Cluster
		for _, cl := range cllist.Items {
			allcls = append(allcls, cl.DeepCopy())
		}

		//	should add identity check here e.g. allcls = utils.FilteClustersByIdentity(r.authClient, instance, allcls)

		for _, cl := range allcls {
			clusters = append(clusters, types.NamespacedName{Name: cl.Name, Namespace: cl.Namespace})
		}
	}

	if err != nil {
		klog.Error("Failed in finding cluster namespaces. error: ", err)
		return nil, err
	}

	klog.V(10).Info("Deploying to clusters", clusters)
	return clusters, nil
}

func (r *ReconcileDeployable) getClustersFromPlacementRef(instance *appv1alpha1.Deployable) ([]types.NamespacedName, error) {
	if klog.V(utils.QuiteLogLel) {
		fnName := utils.GetFnName()
		klog.Infof("Entering: %v()", fnName)
		defer klog.Infof("Exiting: %v()", fnName)
	}

	var clusters []types.NamespacedName
	// only support mcm placementpolicy now
	pp := &placementv1alpha1.PlacementRule{}
	pref := instance.Spec.Placement.PlacementRef
	if len(pref.Kind) > 0 && pref.Kind != "PlacementRule" || len(pref.APIVersion) > 0 && pref.APIVersion != "app.ibm.com/v1alpha1" {
		klog.Warning("Unsupported placement reference:", instance.Spec.Placement.PlacementRef)
		return nil, nil
	}

	klog.V(10).Info("Referencing existing PlacementRule:", instance.Spec.Placement.PlacementRef, " in ", instance.GetNamespace())

	// get placementpolicy resource
	err := r.Get(context.TODO(), client.ObjectKey{Name: instance.Spec.Placement.PlacementRef.Name, Namespace: instance.GetNamespace()}, pp)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warning("Failed to locate placement reference", instance.Spec.Placement.PlacementRef)
			return nil, nil
		}
		return nil, err
	}

	klog.V(10).Info("Preparing cluster namespaces from ", pp)

	for _, decision := range pp.Status.Decisions {
		cluster := types.NamespacedName{Name: decision.ClusterName, Namespace: decision.ClusterNamespace}
		clusters = append(clusters, cluster)
	}

	return clusters, nil
}
