package models

// Rule is a type of validation
type Rule string

// ServiceAccountIsRoleBound is a type for checking whether service account is bounded to certain role or policy rules
var ServiceAccountIsRoleBound Rule

// RBACRules is a bundle of data, needed to check rbac rules
type RBACRules struct {
	Data interface{}
	Type Rule
}
