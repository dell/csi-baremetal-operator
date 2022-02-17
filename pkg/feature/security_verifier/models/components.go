package models

// Component is a component name, for which security verification will be applied
type Component string

var (
	// Node represents the node component
	Node Component = "Node"
	// Scheduler represents the scheduler extender component
	Scheduler Component = "Scheduler"
)
