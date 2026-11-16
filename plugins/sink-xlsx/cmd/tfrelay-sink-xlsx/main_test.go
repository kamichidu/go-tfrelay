package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestRunSmokeWritesWorkbook(t *testing.T) {
	input := `{"ir_version":1,"snapshot_kind":"state","resources":[{"type":"test_thing","name":"example","instances":[{"address":"test_thing.example","attributes":{"name":{"state":"known","type":"string","value":"ok"}}}]}]}`
	path := filepath.Join(t.TempDir(), "result.xlsx")
	if err := run(bytes.NewBufferString(input), &bytes.Buffer{}, path); err != nil { t.Fatal(err) }
	f, err := excelize.OpenFile(path)
	if err != nil { t.Fatal(err) }
	defer f.Close()
	value, err := f.GetCellValue("Resources", "D2")
	if err != nil || value != "ok" { t.Fatalf("value=%q err=%v", value, err) }
}

func TestRunRejectsBadInputWithoutReplacingOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "result.xlsx")
	if err := os.WriteFile(path, []byte("previous artifact"), 0600); err != nil { t.Fatal(err) }
	if err := run(bytes.NewBufferString(`{"ir_version":2}`), &bytes.Buffer{}, path); err == nil { t.Fatal("expected bad contract error") }
	got, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	if string(got) != "previous artifact" { t.Fatalf("existing output changed: %q", got) }
}

func TestRunRejectsTrailingJSON(t *testing.T) {
	input := `{"ir_version":1,"snapshot_kind":"state","resources":[]} {}`
	if err := run(bytes.NewBufferString(input), &bytes.Buffer{}, "-"); err == nil { t.Fatal("expected trailing data error") }
}
