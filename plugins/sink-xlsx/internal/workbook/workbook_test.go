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

func TestWriteRejectsCellOverExcelLimit(t *testing.T) {
	data := projection.RelayData{IRVersion: 1, SnapshotKind: "state", Resources: []projection.Resource{{Type: "x", Name: "y", Instances: []projection.Instance{{Address: "x.y", Attributes: map[string]projection.Value{"large": {State: "known", Type: "string", Value: json.RawMessage(strconvQuote(strings.Repeat("x", maxCellRunes+1)))}}}}}}}
	var out bytes.Buffer
	if err := Write(data, &out); err == nil || !strings.Contains(err.Error(), "character cell limit") {
		t.Fatalf("Write error = %v, want Excel cell limit error", err)
	}
}

func TestAddRowRejectsExcelRowLimit(t *testing.T) {
	rows := make([]row, maxDataRows)
	if err := addRow(&rows, row{}); err == nil || !strings.Contains(err.Error(), "data row limit") {
		t.Fatalf("addRow error = %v, want Excel row limit error", err)
	}
}

func strconvQuote(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
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

func TestResourceAddressUsesNormalizedResourceContract(t *testing.T) {
	addresses := []string{
		`aws_instance.example["a[b"]`,
		`aws_instance.example["a\\\"[b"]`,
		`module.parent["x[y]"].module.child[0].aws_instance.example["k[b"]`,
		`aws_instance.example[0]`,
		`aws_instance.example["for_each"]`,
	}
	for _, address := range addresses {
		data := projection.RelayData{IRVersion: 1, SnapshotKind: "planned", Resources: []projection.Resource{{Address: "module.parent.aws_instance.example", Type: "aws_instance", Name: "example", Instances: []projection.Instance{{Address: address, Attributes: map[string]projection.Value{"id": {State: "known", Type: "string", Value: json.RawMessage(`"x"`)}}}}}}}
		rows, err := rowsFor(data)
		if err != nil {
			t.Fatal(err)
		}
		if rows[0].resource != "module.parent.aws_instance.example" || rows[0].instance != address {
			t.Fatalf("address projection = (%q, %q), want normalized resource and original instance (%q)", rows[0].resource, rows[0].instance, address)
		}
	}
}
