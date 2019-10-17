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
	"encoding/json"
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/onsi/gomega"
	"golang.org/x/net/context"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	placementrulev1alpha1 "github.com/IBM/multicloud-operators-placementrule/pkg/apis/app/v1alpha1"
)

var c client.Client

const timeout = time.Second * 5

var (
	dplname = "example-configmap"
	dplns   = "default"
	dplkey  = types.NamespacedName{
		Name:      dplname,
		Namespace: dplns,
	}
)

var (
	endpoint1 = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"name": "endpoint1-ns",
			},
			Name:      "endpoint1-ns",
			Namespace: "endpoint1-ns",
		},
	}
	endpoint1ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint1-ns",
			Namespace: "endpoint1-ns",
		},
	}

	endpoint2 = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"name": "endpoint2-ns",
			},
			Name:      "endpoint2-ns",
			Namespace: "endpoint2-ns",
		},
	}
	endpoint2ns = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "endpoint2-ns",
			Namespace: "endpoint2-ns",
		},
	}

	endpointnss = []*corev1.Namespace{endpoint1ns, endpoint2ns}
	endpoints   = []*clusterv1alpha1.Cluster{endpoint1, endpoint2}
)

var (
	payload = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "payload",
		},
	}
)

func TestPropagate(t *testing.T) {
	var err error

	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	for _, ns := range endpointnss {
		err = c.Create(context.TODO(), ns)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		defer c.Delete(context.TODO(), ns)
	}

	for _, ep := range endpoints {
		err = c.Create(context.TODO(), ep)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}

	g.Expect(err).NotTo(gomega.HaveOccurred())

	placecluster := placementrulev1alpha1.GenericClusterReference{
		Name: endpoint1.GetName(),
	}

	instance := &appv1alpha1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dplname,
			Namespace: dplns,
		},
		Spec: appv1alpha1.DeployableSpec{
			Template: &runtime.RawExtension{
				Object: payload,
			},
			Placement: &placementrulev1alpha1.Placement{
				GenericPlacementFields: placementrulev1alpha1.GenericPlacementFields{
					Clusters: []placementrulev1alpha1.GenericClusterReference{placecluster},
				},
			},
		},
	}

	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var expectedRequest = reconcile.Request{NamespacedName: dplkey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	dpllist := &appv1alpha1.DeployableList{}
	err = c.List(context.TODO(), &client.ListOptions{Namespace: endpoint1.GetNamespace()}, dpllist)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	if len(dpllist.Items) != 1 {
		t.Errorf("Failed to propagate to cluster endpoint1. items: %v", dpllist)
	}

	if len(dpllist.Items) == 1 {
		dpl := dpllist.Items[0]
		expgenname := instance.GetName() + "-"

		if dpl.GetGenerateName() != expgenname {
			t.Errorf("Incorrect generate name of generated deployable. \n\texpect:\t%s\n\tgot:\t%s", expgenname, dpl.GetGenerateName())
		}
	}

	//delete the instance, verify the propagated dpls in the two clusters should be removed
	err = c.Delete(context.TODO(), instance)

	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	time.Sleep(2 * time.Second)

	dpllist = &appv1alpha1.DeployableList{}
	err = c.List(context.TODO(), &client.ListOptions{Namespace: endpoint1.GetNamespace()}, dpllist)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	t.Logf("dpl items: %v", dpllist.Items)

	if len(dpllist.Items) != 0 {
		t.Errorf("Failed to delete propagated deployable in cluster endpoint1. items: %v", dpllist)
	}
}

func TestReconcile(t *testing.T) {
	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	instance := &appv1alpha1.Deployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dplname,
			Namespace: dplns,
		},
		Spec: appv1alpha1.DeployableSpec{
			Template: &runtime.RawExtension{
				Object: payload,
			},
		},
	}
	err = c.Create(context.TODO(), instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	defer c.Delete(context.TODO(), instance)

	var expectedRequest = reconcile.Request{NamespacedName: dplkey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))
}

func TestOverride(t *testing.T) {
	var err error

	g := gomega.NewGomegaWithT(t)

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.

	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	c = mgr.GetClient()

	recFn, requests := SetupTestReconcile(newReconciler(mgr))
	g.Expect(add(mgr, recFn)).NotTo(gomega.HaveOccurred())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	for _, ns := range endpointnss {
		err = c.Create(context.TODO(), ns)
		g.Expect(err).NotTo(gomega.HaveOccurred())

		defer c.Delete(context.TODO(), ns)
	}

	for _, ep := range endpoints {
		err = c.Create(context.TODO(), ep)
		g.Expect(err).NotTo(gomega.HaveOccurred())
	}

	g.Expect(err).NotTo(gomega.HaveOccurred())

	dplobj := &appv1alpha1.Deployable{}

	dpldata, err := ioutil.ReadFile("../../../examples/configmap-hub-deployable.yaml")
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = yaml.Unmarshal(dpldata, dplobj)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = c.Create(context.TODO(), dplobj)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	defer c.Delete(context.TODO(), dplobj)

	var expectedRequest = reconcile.Request{NamespacedName: dplkey}

	g.Eventually(requests, timeout).Should(gomega.Receive(gomega.Equal(expectedRequest)))

	dpllist := &appv1alpha1.DeployableList{}
	err = c.List(context.TODO(), &client.ListOptions{}, dpllist)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	if len(dpllist.Items) != 3 {
		t.Errorf("Failed to propagate to cluster endpoints. items: %v", dpllist)
	}

	for _, dpl := range dpllist.Items {
		if dpl.GetGenerateName() == "" {
			continue
		}

		expgenname := dplobj.GetName() + "-"

		if dpl.GetGenerateName() != expgenname {
			t.Errorf("Incorrect generate name of generated deployable. \n\texpect:\t%s\n\tgot:\t%s", expgenname, dpl.GetGenerateName())
		}
		//verify override
		if dpl.Namespace == "endpoint1-ns" {
			template := &unstructured.Unstructured{}

			json.Unmarshal(dpl.Spec.Template.Raw, template)
			t.Logf("dpl endpoint 1 template data:%#v", template.Object["data"])

			var expectecdData = make(map[string]interface{})
			expectecdData["purpose"] = "for test"

			if !reflect.DeepEqual(expectecdData, template.Object["data"]) {
				t.Errorf("Incorrect deployable data override. expected data: %#v, actual data: %#v", expectecdData, template.Object["data"])
			}
		}

		if dpl.Namespace == "endpoint2-ns" {
			template := &unstructured.Unstructured{}
			json.Unmarshal(dpl.Spec.Template.Raw, template)
			t.Logf("dpl endpoint 2 template data:%#v", template.Object["data"])

			var expectecdData = make(map[string]interface{})
			expectecdData["foo"] = "bar"

			if !reflect.DeepEqual(expectecdData, template.Object["data"]) {
				t.Errorf("Incorrect deployable data override. expected data: %#v, actual data: %#v", expectecdData, template.Object["data"])
			}
		}
	}
}
