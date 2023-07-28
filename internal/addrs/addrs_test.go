package addrs_test

import (
	"testing"

	"github.com/kamichidu/go-tfrelay/addrs"
	"github.com/stretchr/testify/assert"
)

func TestParser(t *testing.T) {
	cases := []struct {
		Source string
		Addrs  addrs.Addrs
	}{
		{
			`aaa.bbb`,
			addrs.Addrs{
				addrs.Addr{Type: "aaa", Name: "bbb"},
			},
		},
		{
			`aaa.bbb["hoge.jp"]`,
			addrs.Addrs{
				addrs.Addr{Type: "aaa", Name: "bbb", IndexKey: "hoge.jp"},
			},
		},
		{
			`aaa.bbb.ccc.ddd`,
			addrs.Addrs{
				addrs.Addr{Type: "aaa", Name: "bbb"},
				addrs.Addr{Type: "ccc", Name: "ddd"},
			},
		},
		{
			`aaa.bbb["hoge.jp"].ccc.ddd`,
			addrs.Addrs{
				addrs.Addr{Type: "aaa", Name: "bbb", IndexKey: "hoge.jp"},
				addrs.Addr{Type: "ccc", Name: "ddd"},
			},
		},
		{
			`aaa.bbb["hoge.jp"].ccc.ddd["fuga.com"]`,
			addrs.Addrs{
				addrs.Addr{Type: "aaa", Name: "bbb", IndexKey: "hoge.jp"},
				addrs.Addr{Type: "ccc", Name: "ddd", IndexKey: "fuga.com"},
			},
		},
		{
			`module.bbb.ccc.ddd`,
			addrs.Addrs{
				addrs.Addr{Module: "module.bbb"},
				addrs.Addr{Type: "ccc", Name: "ddd"},
			},
		},
		{
			`module.bbb["hoge.jp"].ccc.ddd`,
			addrs.Addrs{
				addrs.Addr{Module: `module.bbb["hoge.jp"]`},
				addrs.Addr{Type: "ccc", Name: "ddd"},
			},
		},
		{
			`module.bbb["hoge.jp"].ccc.ddd["fuga.com"]`,
			addrs.Addrs{
				addrs.Addr{Module: `module.bbb["hoge.jp"]`},
				addrs.Addr{Type: "ccc", Name: "ddd", IndexKey: "fuga.com"},
			},
		},
	}
	for _, c := range cases {
		t.Logf("parse %q", c.Source)
		v := addrs.Parse(c.Source)
		t.Logf("out = %v", v)
		assert.Equal(t, c.Addrs, v, "source: %v", c.Source)
	}
}
