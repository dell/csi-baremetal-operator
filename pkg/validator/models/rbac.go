package models

// Rule ...
type Rule string

// ServiceAccountIsRoleBound ...
var ServiceAccountIsRoleBound Rule

// RBACRules ...
type RBACRules struct {
	Data interface{}
	Type Rule
}
