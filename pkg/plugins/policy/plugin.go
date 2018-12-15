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
)

// PluginName is the name reported in error metrics.
const PluginName = "dynamic_enforced"

// Backend filters audit events according to the policy
// trimming them as necessary to match the level
type Backend struct {
	enforcer        *Enforcer
	delegateBackend audit.Backend
}

// NewBackend returns an enforced audit backend that wraps delegate backend.
// Enforced backend automatically runs and shuts down the delegate backend.
func NewBackend(delegate audit.Backend, e *Enforcer) audit.Backend {
	return &Backend{
		enforcer:        e,
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

		e, err := b.enforcer.ImposeRules(attr, event)
		if e == nil {
			continue
		}
		b.delegateBackend.ProcessEvents(e)
	}
	// Returning true regardless of results, since dynamic audit backends
	// can never cause apiserver request to fail.
	return true
}

// String returns a string representation of the backend
func (b Backend) String() string {
	return fmt.Sprintf("%s<%s>", PluginName, b.delegateBackend)
}
