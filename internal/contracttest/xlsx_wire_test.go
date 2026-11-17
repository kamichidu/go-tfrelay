package contracttest

import (
	"encoding/json"
	"os"
	"testing"

	tfrelay "github.com/kamichidu/go-tfrelay"
)

// This fixture mirrors the XLSX module's deliberately duplicated wire structs.
// Keep it dependency-free so the nested module remains independently consumable.
type xlsxRelayData struct {
	IRVersion    int            `json:"ir_version"`
	SnapshotKind string         `json:"snapshot_kind"`
	Resources    []xlsxResource `json:"resources"`
}
type xlsxResource struct {
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	Provider  string         `json:"provider,omitempty"`
	Instances []xlsxInstance `json:"instances"`
}
type xlsxInstance struct {
	Address             string               `json:"address"`
	IndexKey            string               `json:"index_key,omitempty"`
	Attributes          map[string]xlsxValue `json:"attributes"`
	SensitiveAttributes json.RawMessage      `json:"sensitive_attributes"`
}
type xlsxValue struct {
	State     string          `json:"state"`
	Type      string          `json:"type,omitempty"`
	Sensitive bool            `json:"sensitive,omitempty"`
	Value     json.RawMessage `json:"value,omitempty"`
}

func TestXLSXWireFixtureMatchesRootIR(t *testing.T) {
	input := `{"ir_version":1,"snapshot_kind":"planned","resources":[{"type":"aws_instance","name":"example","provider":"registry/hashicorp/aws","instances":[{"address":"module.child.aws_instance.example[\"a[b\"]","index_key":"a[b","attributes":{"secret":{"state":"known","type":"string","sensitive":true,"value":"x"}},"sensitive_attributes":{"secret":true}}]}]}`
	var root tfrelay.RelayData
	var xlsx xlsxRelayData
	if err := json.Unmarshal([]byte(input), &root); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(input), &xlsx); err != nil {
		t.Fatal(err)
	}
	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	xlsxJSON, err := json.Marshal(xlsx)
	if err != nil {
		t.Fatal(err)
	}
	if !equalJSON(rootJSON, xlsxJSON) {
		t.Fatalf("root and XLSX wire shapes differ:\n%s\n%s", rootJSON, xlsxJSON)
	}
}

func TestGoldenExpectedContainsNoResourceAddress(t *testing.T) {
	paths := []string{
		"../../testdata/golden/snapshots/terraform-1.17.1/create/expected.json",
		"../../testdata/golden/snapshots/terraform-1.17.1/state/expected.json",
		"../../testdata/golden/snapshots/terraform-1.17.1/update/expected.json",
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var relay tfrelay.RelayData
		if err := json.Unmarshal(data, &relay); err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(relay)
		if err != nil {
			t.Fatal(err)
		}
		if !equalJSON(data, encoded) {
			t.Fatalf("%s is not canonical v1 IR", path)
		}
	}
}

func equalJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	left, _ := json.Marshal(x)
	right, _ := json.Marshal(y)
	return string(left) == string(right)
}
