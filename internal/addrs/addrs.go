package addrs

import (
	"fmt"
	"strings"
)

//go:generate peg -noast -switch -inline -strict -output addrs_gen.go addrs.peg

type Addrs []Addr

func (l Addrs) Module() string {
	if n := len(l); n == 0 {
		return ""
	} else if n == 1 {
		// maybe "", it means root module
		return l[0].Module
	}

	modules := make([]string, 0, len(l)-1)
	for i := range l[:len(l)-1] {
		modules = append(modules, l[i].Module)
	}
	return strings.Join(modules, ".")
}

func (l Addrs) Type() string {
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1].Type
}

func (l Addrs) Name() string {
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1].Name
}

func (l Addrs) IndexKey() string {
	if len(l) == 0 {
		return ""
	}
	return l[len(l)-1].IndexKey
}

func (l Addrs) String() string {
	out := make([]string, len(l))
	for i := range l {
		out[i] = l[i].String()
	}
	return strings.Join(out, ".")
}

type Addr struct {
	Module string

	Type string

	Name string

	IndexKey string
}

func (v *Addr) String() string {
	if v.Module != "" {
		return v.Module
	}
	if v.IndexKey == "" {
		return fmt.Sprintf("%q.%q", v.Type, v.Name)
	} else {
		return fmt.Sprintf("%q.%q[%q]", v.Type, v.Name, v.IndexKey)
	}
}

func Parse(s string) Addrs {
	p := &addrsParser{
		Buffer: s,
	}
	if err := p.Init(); err != nil {
		panic(err)
	}
	if err := p.Parse(); err != nil {
		panic(err)
	}
	return p.Out
}

func (p *addrsParser) pushModule() {
	suffix := p.v.IndexKey
	if suffix != "" {
		suffix = `[` + suffix + `]`
	}
	p.Out = append(p.Out, Addr{
		Module: "module." + p.v.Name + suffix,
	})
	p.v = Addr{}
}

func (p *addrsParser) pushResource() {
	p.Out = append(p.Out, p.v)
	p.v = Addr{}
}

func (p *addrsParser) setResourceType(s string) {
	p.v.Type = s
}

func (p *addrsParser) setResourceName(s string) {
	p.v.Name = s
}

func (p *addrsParser) setIndexKey(s string) {
	p.v.IndexKey = s
}
