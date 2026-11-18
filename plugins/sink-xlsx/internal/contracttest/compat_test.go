package contracttest

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestXLSXSinkAgainstPublishedRelayDataFixture(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
	module := filepath.Join(root, "plugins", "sink-xlsx")
	input, err := os.ReadFile(filepath.Join(root, "testdata", "compat", "v1", "sink.input.json"))
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(t.TempDir(), "compat.xlsx")
	cmd := exec.Command("go", "run", "./cmd/tfrelay-sink-xlsx")
	cmd.Dir = module
	cmd.Env = append(os.Environ(), "GOWORK=off", "TFRELAY_SINK_XLSX_OUTPUT="+output)
	cmd.Stdin = bytes.NewReader(input)
	if result, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run XLSX sink: %v\n%s", err, result)
	}
	f, err := excelize.OpenFile(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows("Resources")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) >= 4 {
			got[row[1]] = row[2] + "/" + row[3]
		}
	}
	want := map[string]string{"name": "ok/known", "secret": "[REDACTED]/known", "pending": "<unknown>/unknown"}
	if len(got) != len(want) {
		t.Fatalf("rows = %#v", got)
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %q, want %q", key, got[key], value)
		}
	}
}
