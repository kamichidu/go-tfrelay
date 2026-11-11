package main

import (
	"bytes"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"github.com/zclconf/go-cty/cty"
	ctymsgpack "github.com/zclconf/go-cty/cty/msgpack"
)

func TestMustDecodeMsgpackTerraformDynamicValues(t *testing.T) {
	input := cty.ObjectVal(map[string]cty.Value{
		"count":   cty.NumberIntVal(7),
		"message": cty.StringVal("terraform plan fixture"),
	})
	inputBytes, err := ctymsgpack.Marshal(input, cty.DynamicPseudoType)
	if err != nil {
		t.Fatal(err)
	}

	var encoded bytes.Buffer
	enc := msgpack.NewEncoder(&encoded)
	if err := enc.EncodeMapLen(1); err != nil {
		t.Fatal(err)
	}
	if err := enc.EncodeString("input"); err != nil {
		t.Fatal(err)
	}
	if err := enc.Encode(msgpack.RawMessage(inputBytes)); err != nil {
		t.Fatal(err)
	}

	got := mustDecodeMsgpack(encoded.Bytes())
	if !got.GetAttr("input").RawEquals(input) {
		t.Fatalf("decoded input = %#v, want %#v", got.GetAttr("input"), input)
	}
}

func TestMustDecodeMsgpackLegacyValues(t *testing.T) {
	want := cty.ObjectVal(map[string]cty.Value{
		"name": cty.StringVal("legacy"),
		"pair": cty.TupleVal([]cty.Value{
			cty.StringVal("first"),
			cty.StringVal("second"),
		}),
	})
	encoded, err := ctymsgpack.Marshal(want, want.Type())
	if err != nil {
		t.Fatal(err)
	}

	got := mustDecodeMsgpack(encoded)
	if !got.RawEquals(want) {
		t.Fatalf("decoded value = %#v, want %#v", got, want)
	}
}
