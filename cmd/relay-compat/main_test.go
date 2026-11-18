package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaOnlyDoesNotRequirePlugin(t *testing.T) {
	if err := run("../../schema/v1/relaydata.schema.json", "../../testdata/compat/v1/relaydata.valid.json", "transform", "", "", nil); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaRejectsInvalidFixture(t *testing.T) {
	schema, err := compileSchema("../../schema/v1/relaydata.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../testdata/compat/v1/relaydata.invalid.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validate(schema, data); err == nil {
		t.Fatal("invalid fixture accepted")
	}
}

func TestTransformUsesExplicitExternalFixturePaths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	expected := filepath.Join(dir, "expected.json")
	fixture, err := os.ReadFile("../../testdata/compat/v1/relaydata.valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expected, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(dir, "plugin.sh")
	if err := os.WriteFile(plugin, []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := run("../../schema/v1/relaydata.schema.json", "", "transform", input, expected, []string{plugin}); err != nil {
		t.Fatal(err)
	}
}

func TestTransformRejectsWrongSemanticGolden(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	want := filepath.Join(dir, "expected.json")
	fixture, err := os.ReadFile("../../testdata/compat/v1/transform-groupby.input.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	wrong, err := os.ReadFile("../../testdata/compat/v1/relaydata.valid.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, wrong, 0600); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(dir, "plugin.sh")
	if err := os.WriteFile(plugin, []byte("#!/bin/sh\ncat\n"), 0700); err != nil {
		t.Fatal(err)
	}
	err = run("../../schema/v1/relaydata.schema.json", "", "transform", input, want, []string{plugin})
	if err == nil || !strings.Contains(err.Error(), "semantic golden mismatch") {
		t.Fatalf("expected semantic golden mismatch, got %v", err)
	}
}

func TestSinkDoesNotClaimGenericArtifactGolden(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "sink.json")
	fixture, err := os.ReadFile("../../testdata/compat/v1/sink.input.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(input, fixture, 0600); err != nil {
		t.Fatal(err)
	}
	plugin := filepath.Join(dir, "sink.sh")
	if err := os.WriteFile(plugin, []byte("#!/bin/sh\ncat >/dev/null\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := run("../../schema/v1/relaydata.schema.json", "", "sink", input, "", []string{plugin}); err != nil {
		t.Fatal(err)
	}
}
