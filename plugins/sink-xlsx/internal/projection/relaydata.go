package projection

import "encoding/json"

// RelayData is XLSX's local wire decoder, independent of the root module.
type RelayData struct {
	IRVersion    int        `json:"ir_version"`
	SnapshotKind string     `json:"snapshot_kind"`
	Resources    []Resource `json:"resources"`
}

type Resource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Provider  string     `json:"provider,omitempty"`
	Instances []Instance `json:"instances"`
}

type Instance struct {
	Address             string           `json:"address"`
	IndexKey            string           `json:"index_key,omitempty"`
	Attributes          map[string]Value `json:"attributes"`
	SensitiveAttributes json.RawMessage  `json:"sensitive_attributes"`
}

type Value struct {
	State     string          `json:"state"`
	Type      string          `json:"type,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}
