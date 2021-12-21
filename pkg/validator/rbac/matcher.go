package rbac

import (
	rbacv1 "k8s.io/api/rbac/v1"
)

// Matcher is a helper for matching actual resources with requested ones
type Matcher interface {
	MatchPolicyRules(actual, requested []rbacv1.PolicyRule) (matches bool)
	MatchPolicyRule(actual, requested *rbacv1.PolicyRule) (matches bool)
	MatchRoleBindingsSubjects(roleBindings []rbacv1.RoleBinding, subjectName, namespace string) (matchesRoleBindings []rbacv1.RoleBinding)
	MatchRoleBindingSubjects(roleBinding *rbacv1.RoleBinding, subjectName, namespace string) (matches bool)
	MatchRoles(roles []rbacv1.Role, names []string) (matchesRoles []rbacv1.Role)
}

type matcher struct{}

func (m *matcher) MatchPolicyRules(actual, requested []rbacv1.PolicyRule) (matches bool) {
	for i := 0; i < len(actual); i++ {
		for j := 0; j < len(requested); j++ {
			if matches = m.MatchPolicyRule(&actual[i], &requested[j]); !matches {
				matches = false
				break
			}
		}

		if matches {
			return true
		}
	}
	return false
}

func (m *matcher) MatchPolicyRule(actual, requested *rbacv1.PolicyRule) (matches bool) {
	preparedActualVerbs := make(map[string]struct{})
	for i := 0; i < len(actual.Verbs); i++ {
		preparedActualVerbs[actual.Verbs[i]] = struct{}{}
	}
	preparedActualAPIGroups := make(map[string]struct{})
	for i := 0; i < len(actual.APIGroups); i++ {
		preparedActualAPIGroups[actual.APIGroups[i]] = struct{}{}
	}
	preparedActualResources := make(map[string]struct{})
	for i := 0; i < len(actual.Resources); i++ {
		preparedActualResources[actual.Resources[i]] = struct{}{}
	}
	preparedActualResourceNames := make(map[string]struct{})
	for i := 0; i < len(actual.ResourceNames); i++ {
		preparedActualResourceNames[actual.ResourceNames[i]] = struct{}{}
	}

	for i := 0; i < len(requested.Verbs); i++ {
		if _, ok := preparedActualVerbs[requested.Verbs[i]]; !ok {
			return false
		}
	}

	for i := 0; i < len(requested.APIGroups); i++ {
		if _, ok := preparedActualAPIGroups[requested.APIGroups[i]]; !ok {
			return false
		}
	}

	for i := 0; i < len(requested.Resources); i++ {
		if _, ok := preparedActualResources[requested.Resources[i]]; !ok {
			return false
		}
	}

	for i := 0; i < len(requested.ResourceNames); i++ {
		if _, ok := preparedActualResourceNames[requested.ResourceNames[i]]; !ok {
			return false
		}
	}

	return true
}

func (m *matcher) MatchRoleBindingsSubjects(
	roleBindings []rbacv1.RoleBinding, subjectName, namespace string,
) (matchesRoleBindings []rbacv1.RoleBinding) {
	for i := 0; i < len(roleBindings); i++ {
		if m.MatchRoleBindingSubjects(&roleBindings[i], subjectName, namespace) {
			matchesRoleBindings = append(matchesRoleBindings, roleBindings[i])
		}
	}
	return matchesRoleBindings
}

func (m *matcher) MatchRoleBindingSubjects(
	roleBinding *rbacv1.RoleBinding, subjectName, namespace string,
) (matches bool) {
	for _, subject := range roleBinding.Subjects {
		if subject.Name == subjectName && subject.Namespace == namespace {
			return true
		}
	}
	return false
}

func (m *matcher) MatchRoles(roles []rbacv1.Role, names []string) (matchesRoles []rbacv1.Role) {
	preparedNames := make(map[string]struct{})
	for i := 0; i < len(names); i++ {
		preparedNames[names[i]] = struct{}{}
	}

	for i := 0; i < len(roles); i++ {
		if _, ok := preparedNames[roles[i].Name]; ok {
			matchesRoles = append(matchesRoles, roles[i])
		}
	}
	return
}

// NewMatcher is a constructor for matcher
func NewMatcher() Matcher {
	return &matcher{}
}
