/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package policy

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	auditinstall "k8s.io/apiserver/pkg/apis/audit/install"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/audit"
	"k8s.io/klog"
)

// PluginName is the name reported in error metrics.
const PluginName = "pretty_printer"

// Backend filters audit events according to the policy
// trimming them as necessary to match the level
type Backend struct {
	encoder runtime.Encoder
}

// NewBackend returns a new backend
func NewBackend() audit.Backend {
	scheme := runtime.NewScheme()
	auditinstall.Install(scheme)
	codecs := serializer.NewCodecFactory(scheme)
	yamlSerializer := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme, scheme)
	encoder := codecs.EncoderForVersion(yamlSerializer, auditv1.SchemeGroupVersion)
	return &Backend{
		encoder: encoder,
	}
}

// Run the delegate backend
func (b Backend) Run(stopCh <-chan struct{}) error {
	return nil
}

// Shutdown the delegate backend
func (b Backend) Shutdown() {}

// ProcessEvents enforces policy on a shallow copy of the given event
// dropping any sections that don't conform
func (b Backend) ProcessEvents(events ...*auditinternal.Event) bool {
	for _, event := range events {
		err := b.encoder.Encode(event, os.Stdout)
		if err != nil {
			klog.Fatalf("could not encode audit event: %v", err)
		}
		fmt.Printf("\n----------\n\n")
	}
	return true
}

// String returns a string representation of the backend
func (b Backend) String() string {
	return fmt.Sprintf("%s", PluginName)
}
