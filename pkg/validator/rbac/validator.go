package rbac

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/dell/csi-baremetal-operator/pkg/validator/rbac/models"
)

// Validator is rbac validator for checking predefined rule (e.g. service account is bound to role with certain policy rules)
type Validator interface {
	ValidateServiceAccountIsBound(ctx context.Context, rules *models.ServiceAccountIsRoleBoundData) error
}

type rbac struct {
	client  client.Client
	log     *logrus.Entry
	matcher Matcher
}

func (r *rbac) ValidateServiceAccountIsBound(ctx context.Context, rules *models.ServiceAccountIsRoleBoundData) (err error) {
	// obtaining role bindings for current namespace
	roleBindings := rbacv1.RoleBindingList{}
	if err = r.client.List(ctx, &roleBindings, &client.ListOptions{
		Namespace: rules.Namespace,
	}); err != nil {
		r.log.Errorf("failed to get roles list: %s", err.Error())
		return
	}

	// check if there exists role bindings, which matches passed service account
	matchesRoleBindings := r.matcher.MatchRoleBindingsSubjects(roleBindings.Items, rules.ServiceAccountName, rules.Namespace)
	if len(matchesRoleBindings) == 0 {
		return NewRBACError(fmt.Sprintf("service account not matched, service account: '%s', namespace: '%s'",
			rules.ServiceAccountName, rules.Namespace))
	}

	// obtaining roles for current namespace
	roles := rbacv1.RoleList{}
	if err = r.client.List(ctx, &roles, &client.ListOptions{
		Namespace: rules.Namespace,
	}); err != nil {
		r.log.Errorf("failed to get roles list: %s", err.Error())
		return
	}

	// preparing founded role bindings refs and finding matched ones between them
	matchesRoleBindingsRefs := make([]string, len(matchesRoleBindings))
	for i := 0; i < len(matchesRoleBindings); i++ {
		matchesRoleBindingsRefs[i] = matchesRoleBindings[i].RoleRef.Name
	}
	matchesRoles := r.matcher.MatchRoles(roles.Items, matchesRoleBindingsRefs)
	if len(matchesRoles) == 0 {
		return NewRBACError(fmt.Sprintf("roles not matched, service account: '%s', namespace: '%s'",
			rules.ServiceAccountName, rules.Namespace))
	}

	// matching requested policies between obtained roles
	for i := 0; i < len(matchesRoles); i++ {
		if rules.Role.Name != "" && rules.Role.Name != matchesRoles[i].Name {
			continue
		}
		if rules.Role.Namespace != "" && rules.Role.Namespace != matchesRoles[i].Namespace {
			continue
		}
		if r.matcher.MatchPolicyRules(matchesRoles[i].Rules, rules.Role.Rules) {
			return
		}
	}
	return NewRBACError(fmt.Sprintf("failed to find any roles, matched to passed service account, "+
		"service account: '%s', namespace: '%s'", rules.ServiceAccountName, rules.Namespace))
}

// NewValidator is a constructor for rbac validator
func NewValidator(client client.Client, log *logrus.Entry, matcher Matcher) Validator {
	return &rbac{
		client:  client,
		log:     log,
		matcher: matcher,
	}
}
