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

	auditinternal "k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit"
	ev "k8s.io/apiserver/pkg/audit/event"
	"k8s.io/apiserver/pkg/audit/policy"
)

// PluginName is the name reported in error metrics.
const PluginName = "dynamic_enforced"

// Backend filters audit events according to the policy
// trimming them as necessary to match the level
type Backend struct {
	classRules      []ClassRule
	delegateBackend audit.Backend
}

// NewBackend returns an enforced audit backend that wraps delegate backend.
// Enforced backend automatically runs and shuts down the delegate backend.
func NewBackend(delegate audit.Backend, p policy.Checker) audit.Backend {
	return &Backend{
		delegateBackend: delegate,
	}
}

// Run the delegate backend
func (b Backend) Run(stopCh <-chan struct{}) error {
	return b.delegateBackend.Run(stopCh)
}

// Shutdown the delegate backend
func (b Backend) Shutdown() {
	b.delegateBackend.Shutdown()
}

// ProcessEvents enforces policy on a shallow copy of the given event
// dropping any sections that don't conform
func (b Backend) ProcessEvents(events ...*auditinternal.Event) bool {
	for _, event := range events {
		if event == nil {
			continue
		}
		attr, err := ev.NewAttributes(event)
		if err != nil {
			audit.HandlePluginError(PluginName, err, event)
			continue
		}
		// the magic happens here
		for _, classRule := range b.classRules {
			// does class rule match?
			if ruleMatches(&classRule.Class.Spec, attr) {
				e0 := *event
				e, err := policy.EnforcePolicy(&e0, classRule.Level, classRule.OmitStages)
				if err != nil {
					audit.HandlePluginError(PluginName, err, event)
					continue
				}

				// should we log potentially duplicates or break on first match or log at the highest level given?
				if e == nil {
					continue
				}
				b.delegateBackend.ProcessEvents(e)
			}
		}

	}
	// Returning true regardless of results, since dynamic audit backends
	// can never cause apiserver request to fail.
	return true
}

// String returns a string representation of the backend
func (b Backend) String() string {
	return fmt.Sprintf("%s<%s>", PluginName, b.delegateBackend)
}
