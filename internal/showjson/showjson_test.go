package showjson

import (
	"encoding/json"
	"testing"
)

func TestResourceBlockAddressWithQuotedInstanceKeys(t *testing.T) {
	for _, tc := range []struct{ address, want string }{
		{`aws_x.item["key.with.dots"]`, `aws_x.item`},
		{`aws_x.item["key.with[brackets]"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\\\backslash"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\\"quote"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\\\backslash"]`, `aws_x.item`},
		{`aws_x.item["key.with.escaped\\\"quote"]`, `aws_x.item`},
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

func mustParseValue(t *testing.T, raw string) jsonValue {
	t.Helper()
	got, err := parseValue(json.RawMessage(raw))
	if err != nil {
		t.Fatal(err)
	}
	return got
}
