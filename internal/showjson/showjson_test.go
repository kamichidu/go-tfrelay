package showjson

import "testing"

func TestResourceBlockAddressWithQuotedInstanceKeys(t *testing.T) {
	for _, tc := range []struct{ address, want string }{
		{`aws_x.item["key.with.dots"]`, `aws_x.item`},
		{`aws_x.item["key.with[brackets]"]`, `aws_x.item`},
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
