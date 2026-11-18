// relay-compat validates RelayData and runs a plugin against semantic fixtures.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	schemaPath := flag.String("schema", "schema/v1/relaydata.schema.json", "RelayData JSON Schema")
	fixtures := flag.String("fixtures", "testdata/compat/v1", "compatibility fixture directory")
	input := flag.String("input", "", "single RelayData file to schema-check")
	stage := flag.String("stage", "transform", "plugin stage: transform or pass")
	expected := flag.String("expected", "", "expected stdout file (defaults to stage fixture)")
	flag.Parse()
	if err := run(*schemaPath, *fixtures, *input, *stage, *expected, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "relay-compat:", err)
		os.Exit(1)
	}
}

func run(schemaPath, fixtures, input, stage, expected string, args []string) error {
	if len(args) == 0 {
		return errors.New("plugin command is required")
	}
	compiled, err := compileSchema(schemaPath)
	if err != nil {
		return err
	}
	if input != "" {
		data, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		return validate(compiled, data)
	}
	if stage != "transform" && stage != "pass" {
		return fmt.Errorf("unknown stage %q", stage)
	}
	base := filepath.Join(fixtures, "transform-groupby")
	inPath, wantPath := base+".input.json", base+".expected.json"
	if stage == "pass" {
		wantPath = inPath
	}
	if expected != "" {
		wantPath = expected
	}
	in, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read fixture: %w", err)
	}
	if err := validate(compiled, in); err != nil {
		return fmt.Errorf("fixture schema conformance: %w", err)
	}
	if args[0] == "go" || args[0] == "go.exe" {
		return errors.New("pass a built plugin executable; compile it in the external repository before running relay-compat")
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("plugin failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stage == "transform" || stage == "pass" {
		if err := validate(compiled, stdout.Bytes()); err != nil {
			return fmt.Errorf("plugin output schema conformance: %w", err)
		}
	}
	want, err := os.ReadFile(wantPath)
	if err != nil {
		return fmt.Errorf("read expected output: %w", err)
	}
	if stage == "transform" || stage == "pass" {
		gotCanonical, err := canonical(stdout.Bytes())
		if err != nil {
			return fmt.Errorf("decode plugin output: %w", err)
		}
		wantCanonical, err := canonical(want)
		if err != nil {
			return fmt.Errorf("decode expected output: %w", err)
		}
		if !bytes.Equal(gotCanonical, wantCanonical) {
			return fmt.Errorf("semantic golden mismatch\nwant: %s\n got: %s", wantCanonical, gotCanonical)
		}
	} else if !bytes.Equal(stdout.Bytes(), want) {
		return fmt.Errorf("semantic golden mismatch: got %d output bytes, want %d", stdout.Len(), len(want))
	}
	fmt.Println("schema conformance: passed; semantic golden: passed")
	return nil
}

func compileSchema(path string) (*jsonschema.Schema, error) {
	compiler := jsonschema.NewCompiler()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("open schema: %w", err)
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode schema: %w", err)
	}
	if err := compiler.AddResource("relaydata.schema.json", document); err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	schema, err := compiler.Compile("relaydata.schema.json")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}
	return schema, nil
}

func validate(schema *jsonschema.Schema, data []byte) error {
	var value any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("trailing JSON data")
	}
	return schema.Validate(value)
}

func canonical(data []byte) ([]byte, error) {
	var value any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
