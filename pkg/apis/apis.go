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

package apis

import (
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"k8s.io/klog"

	placementruleapis "github.com/IBM/multicloud-operators-placementrule/pkg/apis"
)

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	// add cluster scheme
	if err := clusterv1alpha1.AddToScheme(s); err != nil {
		klog.Error("unable add cluster to scheme", err)
		return err
	}

	// add placementrule scheme
	if err := placementruleapis.AddToScheme(s); err != nil {
		klog.Error("unable add cluster to scheme", err)
		return err
	}

	return AddToSchemes.AddToScheme(s)
}
