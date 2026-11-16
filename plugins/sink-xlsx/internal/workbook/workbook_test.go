package workbook

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kamichidu/go-tfrelay/plugins/sink-xlsx/internal/projection"
	"github.com/xuri/excelize/v2"
)

func TestWriteRedactsNestedSensitiveValuesAndKeepsUnknowns(t *testing.T) {
	secret := projection.Value{State: "known", Type: "string", Sensitive: true, Value: json.RawMessage(`"never expose"`)}
	unknown := projection.Value{State: "unknown"}
	children, err := json.Marshal(map[string]projection.Value{"private": secret, "pending": unknown, "visible": {State: "known", Type: "number", Value: json.RawMessage(`42`)}})
	if err != nil {
		t.Fatal(err)
	}
	array, err := json.Marshal([]projection.Value{secret})
	if err != nil {
		t.Fatal(err)
	}
	attrs := map[string]projection.Value{
		"object": {State: "known", Type: "object", Value: children},
		"list":   {State: "known", Type: "array", Value: array},
	}
	data := projection.RelayData{IRVersion: 1, SnapshotKind: "planned", Resources: []projection.Resource{{Type: "test_thing", Name: "example", Instances: []projection.Instance{{Address: "test_thing.example", Attributes: attrs}}}}}
	var out bytes.Buffer
	if err := Write(data, &out); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Resources")
	if err != nil {
		t.Fatal(err)
	}
	encoded := out.String()
	if strings.Contains(encoded, "never expose") {
		t.Fatal("sensitive value leaked into workbook bytes")
	}
	values := map[string]string{}
	for _, row := range rows[1:] {
		values[row[2]] = row[3] + "/" + row[4]
	}
	if values["object.private"] != "[REDACTED]/known" || values["list[0]"] != "[REDACTED]/known" {
		t.Fatalf("sensitive rows not redacted: %#v", values)
	}
	if values["object.pending"] != "<unknown>/unknown" || values["object.visible"] != "42/known" {
		t.Fatalf("known/unknown values incorrect: %#v", values)
	}
}

func TestWriteIsByteStable(t *testing.T) {
	data := projection.RelayData{IRVersion: 1, SnapshotKind: "state", Resources: []projection.Resource{{Type: "z", Name: "b", Instances: []projection.Instance{{Address: "z.b", Attributes: map[string]projection.Value{"z": {State: "known", Type: "string", Value: json.RawMessage(`"z"`)}, "a": {State: "known", Type: "string", Value: json.RawMessage(`"a"`)}}}}}}}
	var a, b bytes.Buffer
	if err := Write(data, &a); err != nil {
		t.Fatal(err)
	}
	if err := Write(data, &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("identical input produced different workbook bytes")
	}
}
