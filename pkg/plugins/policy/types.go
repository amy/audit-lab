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
	auditcrdv1alpha1 "github.com/pbarker/audit-lab/pkg/apis/audit/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
)

// ClassRule holds and full audit class along with its rules
type ClassRule struct {
	Class  *auditcrdv1alpha1.AuditClass
	Level  audit.Level
	Stages []audit.Stage
}

// NewClassRule creates a new ClassRule
func NewClassRule(class *auditcrdv1alpha1.AuditClass, rule auditcrdv1alpha1.ClassRule) *ClassRule {
	return &ClassRule{
		Class:  class,
		Level:  audit.Level(rule.Level),
		Stages: convertStages(rule.Stages),
	}
}

func convertStages(stages []auditcrdv1alpha1.Stage) []audit.Stage {
	s := []audit.Stage{}
	for _, stage := range stages {
		s = append(s, audit.Stage(stage))
	}
	return s
}
