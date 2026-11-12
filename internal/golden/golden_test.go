package golden

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalPlanPreservesSemanticFields(t *testing.T) {
	input := []byte(`{"timestamp":"2026-01-01T00:00:00Z","planned_values":{"root_module":{"resources":[{"address":"b"},{"address":"a"}]}},"resource_changes":[{"address":"b","change":{"after_unknown":{"id":true},"after_sensitive":{"secret":true}}},{"address":"a"}]}`)
	got, err := CanonicalPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(got, &value); err != nil {
		t.Fatal(err)
	}
	if value["timestamp"] != nil {
		t.Fatalf("timestamp not normalized: %v", value["timestamp"])
	}
	encoded := string(got)
	for _, preserved := range []string{"after_unknown", "after_sensitive", `"address": "a"`, `"address": "b"`} {
		if !bytes.Contains(got, []byte(preserved)) {
			t.Errorf("canonical JSON lost %q: %s", preserved, encoded)
		}
	}
	if bytes.Index(got, []byte(`"address": "a"`)) > bytes.Index(got, []byte(`"address": "b"`)) {
		t.Fatalf("resource arrays not sorted: %s", encoded)
	}
}

func TestCanonicalRelayNormalizesGeneratedIDOnly(t *testing.T) {
	input := []byte(`{"resources":[{"__id__":"random","type":"x","name":"a","instances":[{"index_key":"1","attributes":{"value":1},"sensitive_attributes":[{"type":"get_attr","value":"secret"}]}]}]}`)
	got, err := CanonicalRelay(input)
	if err != nil {
		t.Fatal(err)
	}
	for _, preserved := range []string{`"value": 1`, "sensitive_attributes", "secret"} {
		if !bytes.Contains(got, []byte(preserved)) {
			t.Errorf("canonical JSON lost %q: %s", preserved, got)
		}
	}
	if !bytes.Contains(got, []byte(`<generated>`)) || bytes.Contains(got, []byte(`"__id__": "random"`)) {
		t.Fatalf("generated ID not normalized: %s", got)
	}
}

func TestCanonicalJSONRejectsTrailingData(t *testing.T) {
	for _, input := range []string{`{} {}`, `{} trailing`} {
		t.Run(input, func(t *testing.T) {
			if _, err := CanonicalPlan([]byte(input)); err == nil {
				t.Fatalf("CanonicalPlan accepted trailing data %q", input)
			}
		})
	}
}

func TestTransformGolden(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "testdata", "golden", "snapshots", "transforms", "groupby")
	got := runGoCommand(t, root, "./cmd/tfrelay-transform-groupby", readFile(t, filepath.Join(dir, "input.json")))
	canonical, err := CanonicalRelay(got)
	if err != nil {
		t.Fatal(err)
	}
	want, err := CanonicalRelay(readFile(t, filepath.Join(dir, "expected.json")))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canonical, want) {
		t.Fatalf("Transform output differs from expected.json: %s", canonical)
	}
}

func TestSinkGolden(t *testing.T) {
	root := repoRoot(t)
	dir := filepath.Join(root, "testdata", "golden", "snapshots", "sinks", "text")
	cmd := exec.Command("go", "run", "./cmd/tfrelay-sink-text")
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(readFile(t, filepath.Join(dir, "input.json")))
	cmd.Env = append(os.Environ(), "TFRELAY_SINK_TEXT_TEMPLATE="+string(readFile(t, filepath.Join(dir, "template.txt"))))
	got, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("Sink command: %v\n%s", err, exit.Stderr)
		}
		t.Fatal(err)
	}
	if !bytes.Equal(got, readFile(t, filepath.Join(dir, "expected.txt"))) {
		t.Fatalf("Sink output differs: %q", got)
	}
}

func runGoCommand(t *testing.T, root, target string, input []byte) []byte {
	t.Helper()
	cmd := exec.Command("go", "run", target)
	cmd.Dir = root
	cmd.Stdin = bytes.NewReader(input)
	output, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%s: %v\n%s", target, err, exit.Stderr)
		}
		t.Fatal(err)
	}
	return output
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
