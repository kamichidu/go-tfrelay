package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/kamichidu/go-tfrelay"
)

func TestGroupResourcesMatchesCanonicalGoldenAndKeepsProviderIdentity(t *testing.T) {
	var input, expected tfrelay.RelayData
	read := func(path string, target any) {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	read("../../testdata/compat/v1/transform-groupby.input.json", &input)
	read("../../testdata/compat/v1/transform-groupby.expected.json", &expected)
	input.Resources = groupResources(input.Resources)
	encoded, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &input); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, expected) {
		t.Fatalf("groupby output differs from canonical golden:\n got: %#v\nwant: %#v", input, expected)
	}
	if len(input.Resources) != 3 {
		t.Fatalf("different providers or types were merged: got %d resources", len(input.Resources))
	}
	merged := input.Resources[0]
	if merged.Name != "first/second" || len(merged.Instances) != 2 {
		t.Fatalf("same type/provider resources were not merged: %#v", merged)
	}
	if got, want := []string{merged.Instances[0].Address, merged.Instances[1].Address}, []string{"terraform_data.first", "terraform_data.second[\"key\"]"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("merged instance addresses = %q, want %q", got, want)
	}
	if input.Resources[1].Provider != "provider-b" {
		t.Fatalf("different provider was merged into group: %#v", input.Resources[1])
	}
}
