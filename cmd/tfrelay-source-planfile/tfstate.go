package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/kamichidu/go-tfrelay/internal/addrs"
	"github.com/kamichidu/go-tfrelay/internal/planproto"
	"github.com/zclconf/go-cty/cty"
	ctyjson "github.com/zclconf/go-cty/cty/json"
	ctymsgpack "github.com/zclconf/go-cty/cty/msgpack"
)

type tfState struct {
	Resources []tfResource `json:"resources"`
}

type tfResource struct {
	Module string `json:"module,omitempty"`

	Mode string `json:"mode"`

	Type string `json:"type"`

	Name string `json:"name"`

	Provider string `json:"provider"`

	Instances []tfInstance `json:"instances"`
}

type tfInstance struct {
	// string or number
	IndexKey json.RawMessage `json:"index_key,omitempty"`

	Attributes map[string]json.RawMessage `json:"attributes"`

	SensitiveAttributes json.RawMessage `json:"sensitive_attributes"`
}

func (v tfInstance) IndexKeyString() string {
	if !json.Valid(v.IndexKey) {
		panic("invalid indexKey: " + string(v.IndexKey))
	}
	return string(v.IndexKey)
}

func (s *tfState) Apply(change *planproto.ResourceInstanceChange) {
	switch change.Change.Action {
	case planproto.Action_CREATE:
		s.applyCreate(change)
	case planproto.Action_UPDATE:
		s.applyUpdate(change)
	case planproto.Action_DELETE:
		s.applyDelete(change)
	default:
		log.Printf("debug: ignore action %v", change.Change.Action)
	}
}

func (s *tfState) applyCreate(change *planproto.ResourceInstanceChange) {
	addr := addrs.Parse(change.Addr)
	resource, ok := s.findResource(addr)
	if !ok {
		resource.Module = addr.Module()
		resource.Mode = "managed"
		resource.Type = addr.Type()
		resource.Name = addr.Name()
		// TODO: detect correct provider
		resource.Provider = "tfrelay-source-planfile"
	}
	instance, ok := s.findInstance(addr)
	if !ok {
		instance.IndexKey = json.RawMessage(addr.IndexKey())
		instance.Attributes = map[string]json.RawMessage{}
	}
	replaceInstance := ok
	newValue := mustDecodeMsgpack(change.Change.Values[0].Msgpack)

	m := newValue.AsValueMap()
	log.Printf("debug: CREATE %v %v", addr, m)
	for k, v := range m {
		if !v.IsWhollyKnown() {
			continue
		}
		b, err := ctyjson.Marshal(v, v.Type())
		if err != nil {
			panic(err)
		}
		instance.Attributes[k] = json.RawMessage(b)
	}

	if replaceInstance {
		for i := range resource.Instances {
			if resource.Instances[i].IndexKeyString() != instance.IndexKeyString() {
				continue
			}
			resource.Instances[i] = instance
			break
		}
	} else {
		resource.Instances = append(resource.Instances, instance)
	}

	s.putResource(addr, resource)
}

func (s *tfState) applyUpdate(change *planproto.ResourceInstanceChange) {
	addr := addrs.Parse(change.Addr)
	resource, ok := s.findResource(addr)
	if !ok {
		resource.Module = addr.Module()
		resource.Mode = "managed"
		resource.Type = addr.Type()
		resource.Name = addr.Name()
		// TODO: detect correct provider
		resource.Provider = "tfrelay-source-planfile"
	}
	instance, ok := s.findInstance(addr)
	if !ok {
		instance.IndexKey = json.RawMessage(addr.IndexKey())
		instance.Attributes = map[string]json.RawMessage{}
	}
	replaceInstance := ok
	oldValue := mustDecodeMsgpack(change.Change.Values[0].Msgpack)
	newValue := mustDecodeMsgpack(change.Change.Values[1].Msgpack)

	m1 := oldValue.AsValueMap()
	m2 := newValue.AsValueMap()
	log.Printf("debug: UPDATE %v %v\n\t=> %v", addr, m1, m2)
	for k, v := range m2 {
		if !v.IsWhollyKnown() {
			continue
		}
		b, err := ctyjson.Marshal(v, v.Type())
		if err != nil {
			panic(err)
		}
		instance.Attributes[k] = json.RawMessage(b)
	}

	if replaceInstance {
		for i := range resource.Instances {
			if resource.Instances[i].IndexKeyString() != instance.IndexKeyString() {
				continue
			}
			resource.Instances[i] = instance
			break
		}
	} else {
		resource.Instances = append(resource.Instances, instance)
	}

	s.putResource(addr, resource)
}

func (s *tfState) applyDelete(change *planproto.ResourceInstanceChange) {
	addr := addrs.Parse(change.Addr)
	resource, ok := s.findResource(addr)
	if !ok {
		resource.Module = addr.Module()
		resource.Mode = "managed"
		resource.Type = addr.Type()
		resource.Name = addr.Name()
		// TODO: detect correct provider
		resource.Provider = "tfrelay-source-planfile"
	}
	instance, ok := s.findInstance(addr)
	if !ok {
		instance.IndexKey = json.RawMessage(addr.IndexKey())
		instance.Attributes = map[string]json.RawMessage{}
	}
	replaceInstance := ok
	delValue := mustDecodeMsgpack(change.Change.Values[0].Msgpack)

	m := delValue.AsValueMap()
	log.Printf("debug: DELETE %v %v", addr, m)
	for k := range m {
		delete(instance.Attributes, k)
	}

	if replaceInstance {
		for i := range resource.Instances {
			if resource.Instances[i].IndexKeyString() != instance.IndexKeyString() {
				continue
			}
			resource.Instances[i] = instance
			break
		}
	} else {
		resource.Instances = append(resource.Instances, instance)
	}

	s.putResource(addr, resource)
}

func (s *tfState) findInstance(addr addrs.Addrs) (tfInstance, bool) {
	r, ok := s.findResource(addr)
	if !ok {
		return tfInstance{}, false
	}

	addrIndexKey := addr.IndexKey()
	if len(r.Instances) == 1 && addrIndexKey == "" {
		return r.Instances[0], true
	}

	var found []tfInstance
	for _, v := range r.Instances {
		if v.IndexKeyString() != addrIndexKey {
			continue
		}
		found = append(found, v)
	}
	// check for debugging
	if n := len(found); n > 1 {
		panic(fmt.Sprintf("duplicated instances found by addr %s", addr))
	} else if n > 0 {
		return found[0], true
	} else {
		return tfInstance{}, false
	}
}

func (s *tfState) findResource(addr addrs.Addrs) (tfResource, bool) {
	addrModule, addrType, addrName := addr.Module(), addr.Type(), addr.Name()

	var found []tfResource
	for _, r := range s.Resources {
		if r.Module != addrModule || r.Type != addrType || r.Name != addrName {
			continue
		}
		found = append(found, r)
	}
	// check for debugging
	if n := len(found); n > 1 {
		panic(fmt.Sprintf("duplicated resources found by addr %s", addr))
	} else if n > 0 {
		return found[0], true
	} else {
		return tfResource{}, false
	}
}

func (s *tfState) putResource(addr addrs.Addrs, r tfResource) {
	if _, ok := s.findResource(addr); !ok {
		s.Resources = append(s.Resources, r)
		return
	}

	for i := range s.Resources {
		if s.Resources[i].Module != r.Module {
			continue
		}
		if s.Resources[i].Mode != r.Mode {
			continue
		}
		if s.Resources[i].Type != r.Type {
			continue
		}
		if s.Resources[i].Name != r.Name {
			continue
		}
		s.Resources[i] = r
		return
	}
}

func mustDecodeMsgpack(b []byte) cty.Value {
	typ, err := ctymsgpack.ImpliedType(b)
	if err != nil {
		typ = cty.DynamicPseudoType
	}
	val, err := ctymsgpack.Unmarshal(b, typ)
	if err != nil {
		panic(err)
	}
	return val
}

func toHexString(b []byte) string {
	var buffer strings.Builder
	for i := range b {
		buffer.WriteString(fmt.Sprintf("\\x%x", b[i]))
	}
	return buffer.String()
}
