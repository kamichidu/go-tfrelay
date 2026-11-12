package showjson

import (
	"encoding/json"
	"testing"
)

func TestResourceBlockAddressWithQuotedInstanceKeys(t *testing.T) {
	for _, tc := range []struct{ address, want string }{
		{`aws_x.item["key.with.dots"]`, `aws_x.item`},
		{`aws_x.item["key.with[brackets]"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\backslash"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\"quote"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\and\"quote"]`, `aws_x.item`},
		{`aws_x.item["key.with.consecutive\\\\backslashes"]`, `aws_x.item`},
		{`module.parent["a.b"].module.child["c[d]"].aws_x.item["key.with.dots[and]brackets"]`, `module.parent["a.b"].module.child["c[d]"].aws_x.item`},
		{`module.parent[0].aws_x.item`, `module.parent[0].aws_x.item`},
	} {
		t.Run(tc.address, func(t *testing.T) {
			if got := resourceBlockAddress(tc.address); got != tc.want {
				t.Fatalf("resourceBlockAddress(%q) = %q, want %q", tc.address, got, tc.want)
			}
		})
	}
}

func TestMaterializeUnknownMaskPreservesContainerSensitivity(t *testing.T) {
	for _, tc := range []struct {
		name string
		mask jsonValue
	}{
		{name: "object", mask: mustParseValue(t, `{"value":true}`)},
		{name: "array", mask: mustParseValue(t, `[true]`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := materializeUnknownMask(tc.mask, mustParseValue(t, `true`), false)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != "known" || !got.Sensitive {
				t.Fatalf("container sensitivity lost: %+v", got)
			}
		})
	}
}

func TestMaterializeUnknownMaskArraySensitivityInheritance(t *testing.T) {
	tests := []struct {
		name              string
		sensitive         string
		wantParent        bool
		wantChildren      []bool
		wantNestedElement bool
	}{
		{name: "parent sensitive", sensitive: `true`, wantParent: true, wantChildren: []bool{true, true}, wantNestedElement: true},
		{name: "parent non-sensitive", sensitive: `false`, wantChildren: []bool{false, false}},
		{name: "nested child sensitive", sensitive: `[false, true]`, wantChildren: []bool{false, true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unknown := mustParseValue(t, `[[true], true]`)
			got, err := materializeUnknownMask(unknown, mustParseValue(t, tc.sensitive), false)
			if err != nil {
				t.Fatal(err)
			}
			if got.Sensitive != tc.wantParent {
				t.Fatalf("parent Sensitive = %v, want %v", got.Sensitive, tc.wantParent)
			}
			var children []struct {
				Sensitive bool            `json:"Sensitive"`
				Value     json.RawMessage `json:"Value"`
			}
			if err := json.Unmarshal(got.Value, &children); err != nil {
				t.Fatal(err)
			}
			if len(children) != len(tc.wantChildren) {
				t.Fatalf("array children = %+v, want %d children", children, len(tc.wantChildren))
			}
			for i, child := range children {
				if child.Sensitive != tc.wantChildren[i] {
					t.Fatalf("array child %d Sensitive = %v, want %v", i, child.Sensitive, tc.wantChildren[i])
				}
			}
			var nested []struct {
				Sensitive bool `json:"Sensitive"`
			}
			if err := json.Unmarshal(children[0].Value, &nested); err != nil {
				t.Fatal(err)
			}
			if len(nested) != 1 || nested[0].Sensitive != tc.wantNestedElement {
				t.Fatalf("nested array children = %+v, want sensitivity %v", nested, tc.wantNestedElement)
			}
		})
	}
}

func mustParseValue(t *testing.T, raw string) jsonValue {
	t.Helper()
	got, err := parseValue(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return got
}
