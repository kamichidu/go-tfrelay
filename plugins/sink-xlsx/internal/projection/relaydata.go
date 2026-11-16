package projection

import "encoding/json"

// RelayData mirrors the stable JSON contract without importing the root module.
type RelayData struct {
	IRVersion    int        `json:"ir_version"`
	SnapshotKind string     `json:"snapshot_kind"`
	Resources    []Resource `json:"resources"`
}

type Resource struct {
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Instances []Instance `json:"instances"`
}

type Instance struct {
	Address    string           `json:"address"`
	Attributes map[string]Value `json:"attributes"`
}

type Value struct {
	State     string          `json:"state"`
	Type      string          `json:"type,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}
