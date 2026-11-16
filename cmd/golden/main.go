package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kamichidu/go-tfrelay/internal/golden"
	"github.com/kamichidu/go-tfrelay/internal/showjson"
)

func main() {
	update := flag.Bool("update", false, "regenerate Source snapshots from committed Terraform show JSON")
	check := flag.Bool("check", false, "compare Source snapshots against committed Terraform show JSON")
	flag.Parse()
	if *update == *check {
		fatalf("choose exactly one of -check or -update")
	}
	root, err := os.Getwd()
	if err != nil {
		fatalf("get working directory: %v", err)
	}
	if err := run(root, *update); err != nil {
		fatalf("%v", err)
	}
}

func run(root string, update bool) error {
	snapshotRoot := filepath.Join(root, "testdata", "golden", "snapshots")
	environments, err := subdirectories(snapshotRoot)
	if err != nil {
		return err
	}
	for _, environment := range environments {
		base := filepath.Join(snapshotRoot, environment)
		cases, err := subdirectories(base)
		if err != nil {
			return err
		}
		for _, testCase := range cases {
			dir := filepath.Join(base, testCase)
			planPath, statePath, expectedPath := filepath.Join(dir, "plan.json"), filepath.Join(dir, "state.json"), filepath.Join(dir, "expected.json")
			inputPath, planned := planPath, true
			if _, err := os.Stat(planPath); os.IsNotExist(err) {
				inputPath, planned = statePath, false
			}
			if _, err := os.Stat(inputPath); os.IsNotExist(err) {
				continue
			}
			input, err := os.ReadFile(inputPath)
			if err != nil {
				return fmt.Errorf("%s/%s input: %w", environment, testCase, err)
			}
			canonicalInput, err := golden.CanonicalPlan(input)
			if err != nil {
				return err
			}
			relay, err := showjson.DecodeSnapshot(canonicalInput, planned)
			if err != nil {
				return fmt.Errorf("%s/%s normalize input: %w", environment, testCase, err)
			}
			encoded, err := json.MarshalIndent(relay, "", "  ")
			if err != nil {
				return err
			}
			actual, err := golden.CanonicalRelay(append(encoded, '\n'))
			if err != nil {
				return err
			}
			if update {
				if err := os.WriteFile(inputPath, canonicalInput, 0644); err != nil {
					return err
				}
				if err := os.WriteFile(expectedPath, actual, 0644); err != nil {
					return err
				}
				fmt.Printf("updated %s/%s\n", environment, testCase)
				continue
			}
			if err := compareSnapshot(inputPath, canonicalInput); err != nil {
				return fmt.Errorf("%s/%s: %w", environment, testCase, err)
			}
			if err := compareSnapshot(expectedPath, actual); err != nil {
				return fmt.Errorf("%s/%s: %w", environment, testCase, err)
			}
			fmt.Printf("verified %s/%s\n", environment, testCase)
		}
	}
	return nil
}

func subdirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func compareSnapshot(path string, actual []byte) error {
	expected, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("snapshot differs at %s; inspect and run `go run ./cmd/golden -update` to approve an intentional refresh", path)
	}
	return nil
}

func fatalf(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...); os.Exit(1) }
