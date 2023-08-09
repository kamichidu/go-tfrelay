package tfrelay

import "encoding/json"

type RelayData struct {
	Resources []Resource `json:"resources"`
}

type Resource struct {
	// set by source plugin or transform plugin
	ResourceID string `json:"__id__"`

	Type string `json:"type"`

	Name string `json:"name"`

	Provider string `json:"provider"`

	Instances []Instance `json:"instances"`
}

type Instance struct {
	// string or number as json
	IndexKey string `json:"index_key,omitempty"`

	Attributes map[string]json.RawMessage `json:"attributes"`

	SensitiveAttributes json.RawMessage `json:"sensitive_attributes"`
}
