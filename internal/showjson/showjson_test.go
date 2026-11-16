package showjson

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestDecodeSnapshotSensitiveMaskShapeMerges(t *testing.T) {
	tests := []struct {
		name    string
		planned string
		after   string
		want    string
	}{
		{name: "planned true with change object", planned: `true`, after: `{"nested":{"leaf":true}}`, want: `true`},
		{name: "planned object with change true", planned: `{"nested":{"leaf":true}}`, after: `true`, want: `true`},
		{name: "nested true applies to descendants", planned: `{"parent":{"child":true}}`, after: `{"parent":{"child":{"leaf":false}}}`, want: `{"parent":{"child":true}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{
  "planned_values":{"root_module":{"resources":[{"address":"test_item.one","type":"test_item","name":"one","values":{"parent":{"child":{"leaf":"secret"},"sibling":"public"},"other":"also secret"},"sensitive_values":%s}]}},
  "resource_changes":[{"address":"test_item.one","change":{"after_sensitive":%s}}]
}`, tc.planned, tc.after)
			got, err := DecodeSnapshot([]byte(input), true)
			if err != nil {
				t.Fatal(err)
			}
			attrs := got.Resources[0].Instances[0].Attributes
			if tc.want == `true` {
				if !attrs["parent"].Sensitive || (tc.name != "nested true applies to descendants" && !attrs["other"].Sensitive) {
					t.Fatalf("true mask did not propagate: parent=%+v other=%+v", attrs["parent"], attrs["other"])
				}
				var parent map[string]struct {
					Sensitive bool            `json:"Sensitive"`
					Value     json.RawMessage `json:"Value"`
				}
				if err := json.Unmarshal(attrs["parent"].Value, &parent); err != nil {
					t.Fatal(err)
				}
				if !parent["child"].Sensitive || (tc.name != "nested true applies to descendants" && !parent["sibling"].Sensitive) {
					t.Fatalf("parent sensitivity did not inherit to all descendants: %+v", parent)
				}
				return
			}
			if (tc.name != "nested true applies to descendants" && !attrs["parent"].Sensitive) || attrs["other"].Sensitive {
				t.Fatalf("nested masks contaminated siblings: parent=%+v other=%+v", attrs["parent"], attrs["other"])
			}
			var parent map[string]struct {
				Sensitive bool            `json:"Sensitive"`
				Value     json.RawMessage `json:"Value"`
			}
			if err := json.Unmarshal(attrs["parent"].Value, &parent); err != nil {
				t.Fatal(err)
			}
			var child map[string]struct {
				Sensitive bool `json:"Sensitive"`
			}
			if err := json.Unmarshal(parent["child"].Value, &child); err != nil {
				t.Fatal(err)
			}
			if !child["leaf"].Sensitive || parent["sibling"].Sensitive {
				t.Fatalf("nested true mask did not propagate within its parent: parent=%+v child=%+v", parent, child)
			}
		})
	}
}

func TestMergeSensitiveArrayMasks(t *testing.T) {
	tests := []struct {
		name, planned, after, want string
	}{
		{"array conflict", `[true,false]`, `[false,true]`, `[true,true]`},
		{"reverse array conflict", `[false,true]`, `[true,false]`, `[true,true]`},
		{"nested arrays and objects", `[{"items":[true,false]},[false,{"leaf":true}]]`, `[{"items":[false,true]},[true,{"leaf":false}]]`, `[{"items":[true,true]},[true,{"leaf":true}]]`},
		{"uneven lengths", `[false,true]`, `[true,false,true]`, `[true,true,true]`},
		{"parent true dominates", `true`, `[{"leaf":false}]`, `true`},
		{"shape mismatch keeps true", `[{"leaf":true}]`, `[true]`, `[true]`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mergeMaskObjects(mustParseValue(t, tc.planned), mustParseValue(t, tc.after))
			if err != nil {
				t.Fatal(err)
			}
			if maskString(got) != tc.want {
				t.Fatalf("merged mask = %s, want %s", maskString(got), tc.want)
			}
		})
	}
}

func TestMergeSensitiveMaskShapeConflicts(t *testing.T) {
	tests := []struct {
		name, planned, after, want string
	}{
		{"object with array", `{"child":false}`, `[false]`, `true`},
		{"array with object", `[false]`, `{"child":false}`, `true`},
		{"nested object with array", `{"outer":{"child":{"leaf":false},"sibling":false}}`, `{"outer":{"child":[false],"sibling":false}}`, `{"outer":{"child":true,"sibling":false}}`},
		{"false with container", `false`, `{"child":true}`, `{"child":true}`},
		{"parent true and sibling isolation", `{"parent":true,"sibling":false}`, `{"parent":{"child":false},"sibling":false}`, `{"parent":true,"sibling":false}`},
		{"missing side retains object", `{"child":true}`, `null`, `{"child":true}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := mergeMaskObjects(mustParseValue(t, tc.planned), mustParseValue(t, tc.after))
			if err != nil {
				t.Fatal(err)
			}
			if maskString(got) != tc.want {
				t.Fatalf("merged mask = %s, want %s", maskString(got), tc.want)
			}
		})
	}
}

func TestDecodeSnapshotConflictingMaskShapesRedactSource(t *testing.T) {
	for _, tc := range []struct{ name, planned, after string }{
		{"object with array", `{"items":{"secret":false},"public":"visible"}`, `{"items":[false]}`},
		{"array with object", `{"items":[false],"public":"visible"}`, `{"items":{"secret":false}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"planned_values":{"root_module":{"resources":[{"address":"test_item.one","type":"test_item","name":"one","values":{"items":{"secret":"hidden","public":"nested-visible"},"public":"visible"},"sensitive_values":%s}]}},"resource_changes":[{"address":"test_item.one","change":{"after_sensitive":%s}}]}`, tc.planned, tc.after)
			got, err := DecodeSnapshot([]byte(input), true)
			if err != nil {
				t.Fatal(err)
			}
			attrs := got.Resources[0].Instances[0].Attributes
			if !attrs["items"].Sensitive || attrs["public"].Sensitive {
				t.Fatalf("conflict sensitivity = items:%v public:%v", attrs["items"].Sensitive, attrs["public"].Sensitive)
			}
			var descendants map[string]struct {
				Sensitive bool `json:"sensitive"`
			}
			if err := json.Unmarshal(attrs["items"].Value, &descendants); err != nil {
				t.Fatal(err)
			}
			for key, child := range descendants {
				if !child.Sensitive {
					t.Fatalf("conflicting subtree descendant %q lost sensitivity: %s", key, attrs["items"].Value)
				}
			}
		})
	}
}

func TestDecodeSnapshotSensitiveArrayMaskUnionRedactsSource(t *testing.T) {
	input := []byte(`{"planned_values":{"root_module":{"resources":[{"address":"test_item.one","type":"test_item","name":"one","values":{"items":["left-secret","right-secret"],"public":"visible"},"sensitive_values":{"items":[true,false]}}]}},"resource_changes":[{"address":"test_item.one","change":{"after_sensitive":{"items":[false,true]}}}]}`)
	got, err := DecodeSnapshot(input, true)
	if err != nil {
		t.Fatal(err)
	}
	items := got.Resources[0].Instances[0].Attributes["items"]
	var values []struct {
		Sensitive bool   `json:"Sensitive"`
		Value     string `json:"Value"`
	}
	if err := json.Unmarshal(items.Value, &values); err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || !values[0].Sensitive || !values[1].Sensitive {
		t.Fatalf("array sensitivity union = %+v, want both elements sensitive", values)
	}
	if string(items.Value) == "" || got.Resources[0].Instances[0].Attributes["public"].Sensitive {
		t.Fatal("redaction union changed unrelated sibling")
	}
}

func maskString(v jsonValue) string {
	if v.kind == "array" {
		parts := make([]string, len(v.arr))
		for i, child := range v.arr {
			parts[i] = maskString(child)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	if v.kind == "object" {
		keys := make([]string, 0, len(v.obj))
		for key := range v.obj {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, key := range keys {
			encoded, _ := json.Marshal(key)
			parts[i] = string(encoded) + ":" + maskString(v.obj[key])
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	if v.kind == "boolean" {
		if maskTrue(v) {
			return "true"
		}
		return "false"
	}
	return "null"
}

func TestDecodeSnapshotOrdersResourcesAndInstances(t *testing.T) {
	input := []byte(`{
  "values":{"root_module":{"resources":[
    {"address":"z_item.last","type":"z_item","name":"last","values":{"items":["z-first","z-second"]}},
    {"address":"a_item[\"z\"]","type":"a_item","name":"item","index":"z","values":{"value":"z"}},
    {"address":"a_item[\"a\"]","type":"a_item","name":"item","index":"a","values":{"value":"a"}},
    {"address":"a_other.one","type":"a_other","name":"one","values":{"value":"other"}}
  ]}}
}`)
	got, err := DecodeSnapshot(input, false)
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, resource := range got.Resources {
		for _, instance := range resource.Instances {
			order = append(order, instance.Address)
		}
	}
	want := []string{`a_item["a"]`, `a_item["z"]`, "a_other.one", "z_item.last"}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("instance order = %v, want %v", order, want)
	}
	var array []struct {
		Value string `json:"Value"`
	}
	if err := json.Unmarshal(got.Resources[2].Instances[0].Attributes["items"].Value, &array); err != nil {
		t.Fatal(err)
	}
	if len(array) != 2 || array[0].Value != "z-first" || array[1].Value != "z-second" {
		t.Fatalf("array attribute order changed: %+v", array)
	}
}

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
