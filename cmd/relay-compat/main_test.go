package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaAcceptsValidAndRejectsInvalidFixtures(t *testing.T) {
	schema, err := compileSchema("../../schema/v1/relaydata.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile("../../testdata/compat/v1/relaydata.valid.json"); err != nil {
		t.Fatal(err)
	} else if err := validate(schema, data); err != nil {
		t.Fatalf("valid fixture rejected: %v", err)
	}
	if data, err := os.ReadFile("../../testdata/compat/v1/relaydata.invalid.json"); err != nil {
		t.Fatal(err)
	} else if err := validate(schema, data); err == nil {
		t.Fatal("invalid fixture accepted")
	}
}

func TestHarnessRunsPluginAndChecksSemanticGolden(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "plugin.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err := run("../../schema/v1/relaydata.schema.json", "../../testdata/compat/v1", "", "pass", "", []string{script})
	if err != nil {
		t.Fatal(err)
	}
}
