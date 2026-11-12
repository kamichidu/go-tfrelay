package main

import (
	"encoding/json"
	"testing"

	"github.com/kamichidu/go-tfrelay"
)

func TestRedactValueProtectsSensitiveDescendants(t *testing.T) {
	child := map[string]tfrelay.Value{"secret": {State: "known", Type: "string", Sensitive: true, Value: json.RawMessage(`"do-not-print"`)}, "public": {State: "known", Type: "string", Value: json.RawMessage(`"visible"`)}}
	encoded, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	parent := tfrelay.Value{State: "known", Type: "object", Value: encoded}
	got, err := redactValue(parent)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]tfrelay.Value
	if err := json.Unmarshal(got.Value, &values); err != nil {
		t.Fatal(err)
	}
	var secret string
	if err := json.Unmarshal(values["secret"].Value, &secret); err != nil || secret != "<sensitive>" {
		t.Fatalf("secret not redacted: %s", values["secret"].Value)
	}
	if string(values["public"].Value) != `"visible"` {
		t.Fatalf("public value changed: %s", values["public"].Value)
	}
}
