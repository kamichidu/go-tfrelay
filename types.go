package tfrelay

import "encoding/json"

type RelayData struct {
	IRVersion    int        `json:"ir_version"`
	SnapshotKind string     `json:"snapshot_kind"`
	Resources    []Resource `json:"resources"`
}

type Resource struct {
	// set by source plugin or transform plugin
	ResourceID string `json:"__id__"`

	Type string `json:"type"`

	Name string `json:"name"`

	Provider string `json:"provider,omitempty"`

	Instances []Instance `json:"instances"`
}

type Instance struct {
	Address string `json:"address"`
	// string or number as json
	IndexKey string `json:"index_key,omitempty"`

	Attributes map[string]Value `json:"attributes"`

	SensitiveAttributes json.RawMessage `json:"sensitive_attributes"`
}

// Value represents a known JSON value or an unknown Terraform value.
type Value struct {
	State     string          `json:"state"`
	Type      string          `json:"type,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}
