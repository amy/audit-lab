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
