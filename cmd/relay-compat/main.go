// relay-compat validates the RelayData protocol and optionally runs a plugin.
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
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func main() {
	schema := flag.String("schema", "schema/v1/relaydata.schema.json", "RelayData JSON Schema")
	input := flag.String("input", "", "validate this RelayData file only; no plugin command is needed")
	stage := flag.String("stage", "transform", "plugin mode: source, transform, sink, or pass")
	fixture := flag.String("input-fixture", "", "RelayData fixture sent to a transform or sink")
	expected := flag.String("expected", "", "optional expected RelayData stdout file for source/transform/pass")
	flag.Parse()
	if err := run(*schema, *input, *stage, *fixture, *expected, flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "relay-compat:", err)
		os.Exit(1)
	}
}

func run(schemaPath, input, stage, fixture, expected string, args []string) error {
	compiled, err := compileSchema(schemaPath)
	if err != nil {
		return err
	}
	if input != "" {
		data, err := os.ReadFile(input)
		if err != nil {
			return err
		}
		if err := validate(compiled, data); err != nil {
			return err
		}
		fmt.Println("schema conformance: passed")
		return nil
	}
	if stage != "source" && stage != "transform" && stage != "sink" && stage != "pass" {
		return fmt.Errorf("unknown stage %q (want source, transform, sink, or pass)", stage)
	}
	if stage == "sink" && expected != "" {
		return errors.New("-expected is not supported for sink; verify emitted artifacts with plugin-owned tests")
	}
	if len(args) == 0 {
		return errors.New("plugin command is required")
	}
	var in []byte
	if stage == "transform" || stage == "sink" || stage == "pass" {
		if fixture == "" {
			return fmt.Errorf("-%s mode requires -input-fixture", stage)
		}
		in, err = os.ReadFile(fixture)
		if err != nil {
			return fmt.Errorf("read input fixture: %w", err)
		}
		if err := validate(compiled, in); err != nil {
			return fmt.Errorf("fixture schema conformance: %w", err)
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	if in != nil {
		cmd.Stdin = bytes.NewReader(in)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("plugin failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if stage == "sink" {
		fmt.Println("protocol input schema conformance: passed; plugin execution: passed (artifact semantics require plugin-owned tests)")
		return nil
	}
	if err := validate(compiled, stdout.Bytes()); err != nil {
		return fmt.Errorf("plugin output schema conformance: %w", err)
	}
	if expected != "" {
		want, err := os.ReadFile(expected)
		if err != nil {
			return fmt.Errorf("read expected output: %w", err)
		}
		gotCanonical, err := canonical(stdout.Bytes())
		if err != nil {
			return fmt.Errorf("decode plugin output: %w", err)
		}
		wantCanonical, err := canonical(want)
		if err != nil {
			return fmt.Errorf("decode expected output: %w", err)
		}
		if !bytes.Equal(gotCanonical, wantCanonical) {
			return fmt.Errorf("plugin semantic golden mismatch\nwant: %s\n got: %s", wantCanonical, gotCanonical)
		}
		fmt.Println("protocol schema conformance: passed; plugin semantic golden: passed")
		return nil
	}
	fmt.Println("protocol schema conformance: passed; plugin semantic golden: not configured")
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
