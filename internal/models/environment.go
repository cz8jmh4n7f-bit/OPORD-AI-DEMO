package models

// ComponentKind is the resource type a blueprint component expands into.
type ComponentKind string

const (
	ComponentCluster  ComponentKind = "k8s-cluster"
	ComponentVM       ComponentKind = "vm"
	ComponentDatabase ComponentKind = "database"
	// ComponentStack covers any first-class tofu module (S3, Secret, SQS,
	// ElastiCache, etc.) - the workhorse for piping outputs into siblings.
	ComponentStack ComponentKind = "stack"
)

// Component is one piece of a composed environment. Exactly one of
// Cluster / VM / Database / Stack is set, matching Kind.
//
// Spec values can carry placeholders of the form `${other.outputs.field}` to
// pipe outputs from a sibling component into this one (ADR-0008). The
// orchestrator orders components into topological waves and substitutes
// placeholders just before each component is created.
type Component struct {
	Name     string        `json:"name"` // unique within the environment
	Kind     ComponentKind `json:"kind"`
	Cluster  *ClusterSpec  `json:"cluster,omitempty"`
	VM       *VMSpec       `json:"vm,omitempty"`
	Database *DatabaseSpec `json:"database,omitempty"`
	Stack    *StackSpec    `json:"stack,omitempty"`
}

// Blueprint is a golden-path template: a named environment composed of typed
// components. The catalog (internal/templates) holds the built-in blueprints.
type Blueprint struct {
	ID          string      `json:"id"`   // stable identifier, e.g. "web-app"
	Name        string      `json:"name"` // human label
	Description string      `json:"description"`
	Components  []Component `json:"components"`
}

// EnvironmentSpec is the persisted, expanded spec for one environment instance
// (serialized into environments.spec). Components carry their child resource
// specs; each child is named "<environment>-<component.Name>".
type EnvironmentSpec struct {
	Blueprint  string      `json:"blueprint"`
	Provider   string      `json:"provider"`
	Components []Component `json:"components"`
}
