package policy

import (
	"strings"

	auditcrdv1alpha1 "github.com/pbarker/audit-lab/pkg/apis/audit/v1alpha1"
	"k8s.io/apiserver/pkg/apis/audit"
	"k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// Enforcer enforces class rules
type Enforcer struct {
	rules []*ClassRule
}

// NewEnforcer creates a new policy checker.
func NewEnforcer(rules []*ClassRule) *Enforcer {
	return &Enforcer{rules}
}

// ImposeRules applies the rules to an event
func (e *Enforcer) ImposeRules(attrs authorizer.Attributes, event *audit.Event) (*audit.Event, error) {
	// gather all rules that match request
	matchedLevels := []audit.Level{}
	for _, rule := range e.rules {
		if ruleMatches(&rule.Class.Spec, attrs) {
			if stageMatches(event, rule.Stages) {
				matchedLevels = append(matchedLevels, rule.Level)
			}
		}
	}
	if len(matchedLevels) == 0 {
		return nil, nil
	}

	// take the deepest level match
	var finalLevel audit.Level
	for _, level := range matchedLevels {
		if finalLevel.Less(level) {
			finalLevel = level
		}
	}

	ev0 := *event
	ev, err := policy.EnforcePolicy(&ev0, finalLevel, []audit.Stage{})
	if err != nil {
		return nil, err
	}
	return ev, nil
}

// NOTE: much of the code below was copied from apiserver/pkg/audit/policy/checker.go because the methods were not exported.

func stageMatches(event *audit.Event, stages []audit.Stage) bool {
	for _, stage := range stages {
		if stage == event.Stage {
			return true
		}
	}
	return false
}

// Check whether the rule matches the request attrs.
func ruleMatches(r *auditcrdv1alpha1.AuditClassSpec, attrs authorizer.Attributes) bool {
	user := attrs.GetUser()
	if len(r.Users) > 0 {
		if user == nil || !hasString(r.Users, user.GetName()) {
			return false
		}
	}
	if len(r.UserGroups) > 0 {
		if user == nil {
			return false
		}
		matched := false
		for _, group := range user.GetGroups() {
			if hasString(r.UserGroups, group) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if len(r.Verbs) > 0 {
		if !hasString(r.Verbs, attrs.GetVerb()) {
			return false
		}
	}

	if len(r.Namespaces) > 0 || len(r.Resources) > 0 {
		return ruleMatchesResource(r, attrs)
	}

	if len(r.NonResourceURLs) > 0 {
		return ruleMatchesNonResource(r, attrs)
	}

	return true
}

// Check whether the rule's non-resource URLs match the request attrs.
func ruleMatchesNonResource(r *auditcrdv1alpha1.AuditClassSpec, attrs authorizer.Attributes) bool {
	if attrs.IsResourceRequest() {
		return false
	}

	path := attrs.GetPath()
	for _, spec := range r.NonResourceURLs {
		if pathMatches(path, spec) {
			return true
		}
	}

	return false
}

// Check whether the path matches the path specification.
func pathMatches(path, spec string) bool {
	// Allow wildcard match
	if spec == "*" {
		return true
	}
	// Allow exact match
	if spec == path {
		return true
	}
	// Allow a trailing * subpath match
	if strings.HasSuffix(spec, "*") && strings.HasPrefix(path, strings.TrimRight(spec, "*")) {
		return true
	}
	return false
}

// Check whether the rule's resource fields match the request attrs.
func ruleMatchesResource(r *auditcrdv1alpha1.AuditClassSpec, attrs authorizer.Attributes) bool {
	if !attrs.IsResourceRequest() {
		return false
	}

	if len(r.Namespaces) > 0 {
		if !hasString(r.Namespaces, attrs.GetNamespace()) { // Non-namespaced resources use the empty string.
			return false
		}
	}
	if len(r.Resources) == 0 {
		return true
	}

	apiGroup := attrs.GetAPIGroup()
	resource := attrs.GetResource()
	subresource := attrs.GetSubresource()
	combinedResource := resource
	// If subresource, the resource in the policy must match "(resource)/(subresource)"
	if subresource != "" {
		combinedResource = resource + "/" + subresource
	}

	name := attrs.GetName()

	for _, gr := range r.Resources {
		if gr.Group == apiGroup {
			if len(gr.Resources) == 0 {
				return true
			}
			for _, res := range gr.Resources {
				if len(gr.ResourceNames) == 0 || hasString(gr.ResourceNames, name) {
					// match "*"
					if res == combinedResource || res == "*" {
						return true
					}
					// match "*/subresource"
					if len(subresource) > 0 && strings.HasPrefix(res, "*/") && subresource == strings.TrimLeft(res, "*/") {
						return true
					}
					// match "resource/*"
					if strings.HasSuffix(res, "/*") && resource == strings.TrimRight(res, "/*") {
						return true
					}
				}
			}
		}
	}
	return false
}

// Utility function to check whether a string slice contains a string.
func hasString(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}
