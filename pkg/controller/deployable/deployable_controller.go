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
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	appv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Deployable Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	reccs, err := kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		klog.Error("Failed to new clientset for event recorder. err: ", err)
		os.Exit(1)
	}

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: reccs.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(mgr.GetScheme(), corev1.EventSource{Component: "deployable"})

	return add(mgr, newReconciler(mgr, recorder))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, recorder record.EventRecorder) reconcile.Reconciler {
	return &ReconcileDeployable{
		Client:        mgr.GetClient(),
		scheme:        mgr.GetScheme(),
		eventRecorder: recorder,
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
	err = c.Watch(&source.Kind{Type: &appv1alpha1.Deployable{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDeployable implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeployable{}

// ReconcileDeployable reconciles a Deployable object
type ReconcileDeployable struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	scheme *runtime.Scheme

	eventRecorder record.EventRecorder
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
			instance.Status.LastUpdateTime = metav1.Now()
			klog.V(10).Info("Update status", instance.Status)
			statuserr := r.Status().Update(context.TODO(), instance)
			if statuserr != nil {
				klog.Error("Error returned when updating status:", err, "instance:", instance)
				return reconcile.Result{}, statuserr
			}
		}
	}

	klog.V(10).Info("Reconciling - finished.", request.NamespacedName, " with Get err:", err)

	return reconcile.Result{}, nil
}
