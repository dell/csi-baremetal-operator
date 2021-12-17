package models

import v1 "k8s.io/api/rbac/v1"

// ServiceAccountIsRoleBoundData is bundle of data, needed for checking whether service account is bounded
// to certain role or policy rules
type ServiceAccountIsRoleBoundData struct {
	Role               *v1.Role
	ServiceAccountName string
	Namespace          string
}
