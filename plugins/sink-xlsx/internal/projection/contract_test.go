package projection

import (
	"encoding/json"
	"testing"

	tfrelay "github.com/kamichidu/go-tfrelay"
)

func TestProjectionMatchesRootIRJSONWireShape(t *testing.T) {
	root := tfrelay.RelayData{IRVersion: 1, SnapshotKind: "planned", Resources: []tfrelay.Resource{{Address: `module.child.aws_instance.example`, Type: "aws_instance", Name: "example", Provider: "registry/hashicorp/aws", Instances: []tfrelay.Instance{{Address: `module.child.aws_instance.example["a[b"]`, IndexKey: `a[b`, Attributes: map[string]tfrelay.Value{}, SensitiveAttributes: json.RawMessage(`{"password":true}`)}}}}}
	projected := RelayData{IRVersion: 1, SnapshotKind: "planned", Resources: []Resource{{Address: `module.child.aws_instance.example`, Type: "aws_instance", Name: "example", Provider: "registry/hashicorp/aws", Instances: []Instance{{Address: `module.child.aws_instance.example["a[b"]`, IndexKey: `a[b`, Attributes: map[string]Value{}, SensitiveAttributes: json.RawMessage(`{"password":true}`)}}}}}
	rootJSON, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	projectedJSON, err := json.Marshal(projected)
	if err != nil {
		t.Fatal(err)
	}
	var rootShape, projectedShape any
	if err := json.Unmarshal(rootJSON, &rootShape); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(projectedJSON, &projectedShape); err != nil {
		t.Fatal(err)
	}
	if !equalJSON(rootShape, projectedShape) {
		t.Fatalf("projection JSON shape differs from root IR\nroot: %s\nprojection: %s", rootJSON, projectedJSON)
	}
}

func equalJSON(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}
