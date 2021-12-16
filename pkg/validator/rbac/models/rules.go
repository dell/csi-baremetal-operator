package models

import v1 "k8s.io/api/rbac/v1"

// ServiceAccountIsRoleBoundData ...
type ServiceAccountIsRoleBoundData struct {
	Role               *v1.Role
	ServiceAccountName string
	Namespace          string
}
