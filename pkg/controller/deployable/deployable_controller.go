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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	"github.com/IBM/multicloud-operators-deployable/pkg/utils"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Deployable Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	erecorder, _ := utils.NewEventRecorder(mgr.GetConfig(), mgr.GetScheme())

	return &ReconcileDeployable{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		eventRecorder: erecorder,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployable-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Deployable
	err = c.Watch(
		&source.Kind{Type: &appv1alpha1.Deployable{}},
		&handler.EnqueueRequestsFromMapFunc{ToRequests: &deployableMapper{mgr.GetClient()}},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				newdpl := e.ObjectNew.(*appv1alpha1.Deployable)
				olddpl := e.ObjectOld.(*appv1alpha1.Deployable)

				if len(newdpl.GetFinalizers()) > 0 {
					return true
				}

				hosting := utils.GetHostDeployableFromObject(newdpl)
				if hosting != nil {
					// reconcile its parent for status
					return true
				}

				if !reflect.DeepEqual(newdpl.GetAnnotations(), olddpl.GetAnnotations()) {
					return true
				}

				if !reflect.DeepEqual(newdpl.GetLabels(), olddpl.GetLabels()) {
					return true
				}

				oldtmpl := &unstructured.Unstructured{}
				newtmpl := &unstructured.Unstructured{}

				if olddpl.Spec.Template == nil || olddpl.Spec.Template.Raw == nil {
					return true
				}
				err = json.Unmarshal(olddpl.Spec.Template.Raw, oldtmpl)
				if err != nil {
					return true
				}
				if newdpl.Spec.Template.Raw == nil {
					return true
				}
				err = json.Unmarshal(newdpl.Spec.Template.Raw, newtmpl)
				if err != nil {
					return true
				}

				if !reflect.DeepEqual(newtmpl, oldtmpl) {
					return true
				}

				olddpl.Spec.Template = newdpl.Spec.Template.DeepCopy()
				return !reflect.DeepEqual(olddpl.Spec, newdpl.Spec)
			},
		})
	if err != nil {
		return err
	}

	return nil
}

type deployableMapper struct {
	client.Client
}

func (mapper *deployableMapper) Map(obj handler.MapObject) []reconcile.Request {
	if klog.V(utils.QuiteLogLel) {
		fnName := utils.GetFnName()
		klog.Infof("Entering: %v()", fnName)
		defer klog.Infof("Exiting: %v()", fnName)
	}

	// rolling target deployable changed, need to update the rolling deployable
	var requests []reconcile.Request

	// enqueue itself
	requests = append(requests,
		reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      obj.Meta.GetName(),
				Namespace: obj.Meta.GetNamespace(),
			},
		},
	)

	// list thing for rolling update check
	dplList := &appv1alpha1.DeployableList{}
	listopts := &client.ListOptions{Namespace: obj.Meta.GetNamespace()}
	err := mapper.List(context.TODO(), listopts, dplList)
	if err != nil {
		klog.Error("Listing deployables in mapper and got error:", err)
	}

	for _, dpl := range dplList.Items {
		annotations := dpl.GetAnnotations()
		if annotations == nil || annotations[appv1alpha1.AnnotationRollingUpdateTarget] == "" {
			// not rolling
			continue
		}

		if annotations[appv1alpha1.AnnotationRollingUpdateTarget] != obj.Meta.GetName() {
			// rolling to annother one, skipping
			continue
		}

		// rolling target deployable changed, need to update the rolling deployable
		objkey := types.NamespacedName{
			Name:      dpl.GetName(),
			Namespace: dpl.GetNamespace(),
		}
		requests = append(requests, reconcile.Request{NamespacedName: objkey})
	}
	// end of rolling update check

	// reconcile hosting one, if there is change in cluster, assuming no 2-hop hosting
	hdplkey := utils.GetHostDeployableFromObject(obj.Meta)
	if hdplkey != nil && hdplkey.Name != "" {
		requests = append(requests, reconcile.Request{NamespacedName: *hdplkey})
	}

	klog.V(10).Info("Out deployable mapper with requests:", requests)

	return requests
}

// blank assignment to verify that ReconcileDeployable implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeployable{}

// ReconcileDeployable reconciles a Deployable object
type ReconcileDeployable struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme

	eventRecorder *utils.EventRecorder
}

// Reconcile reads that state of the cluster for a Deployable object and makes changes based on the state read
// and what is in the Deployable.Spec
func (r *ReconcileDeployable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the Deployable instance

	instance := &appv1alpha1.Deployable{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	klog.Info("Reconciling:", request.NamespacedName, " with Get err:", err)

	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			klog.V(10).Info("Reconciling - finished.", request.NamespacedName, " with Get err:", err)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		klog.V(10).Info("Reconciling - finished.", request.NamespacedName, " with Get err:", err)
		return reconcile.Result{}, err
	}

	savedStatus := instance.Status.DeepCopy()

	// try if it is a hub deployable
	err = r.handleDeployable(instance)
	if err != nil {
		instance.Status.Phase = appv1alpha1.DeployableFailed
		instance.Status.PropagatedStatus = nil
		instance.Status.Reason = err.Error()
	} else {
		instance.Status.Reason = ""
		instance.Status.Message = ""
	}

	// reconcile finished check if need to upadte the resource
	if len(instance.GetObjectMeta().GetFinalizers()) == 0 {
		if !reflect.DeepEqual(savedStatus, instance.Status) {
			now := metav1.Now()
			instance.Status.LastUpdateTime = &now
			klog.V(10).Info("Update status", instance.Status)
			err = r.Status().Update(context.TODO(), instance)
			if err != nil {
				klog.Error("Error returned when updating status:", err, "instance:", instance)
				return reconcile.Result{}, err
			}
		}
	}

	klog.V(10).Info("Reconciling - finished.", request.NamespacedName, " with Get err:", err)

	return reconcile.Result{}, nil
}
