package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kamichidu/go-tfrelay/internal/golden"
)

type importSpec struct {
	Address string `json:"address"`
	ID      string `json:"id"`
}

func main() {
	update := flag.Bool("update", false, "regenerate committed Terraform and Source snapshots")
	check := flag.Bool("check", false, "regenerate snapshots in temporary directories and compare them")
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
	goldenRoot := filepath.Join(root, "testdata", "golden")
	terraformRoot := filepath.Join(goldenRoot, "environments")
	casesRoot := filepath.Join(goldenRoot, "cases")
	snapshotRoot := filepath.Join(goldenRoot, "snapshots")
	terraformVersion, err := terraformVersion()
	if err != nil {
		return err
	}
	tempRoot, err := os.MkdirTemp("", "tfrelay-golden-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempRoot)
	sourceBinary := filepath.Join(tempRoot, "tfrelay-source-planfile")
	if err := runCommand(root, "go", "build", "-o", sourceBinary, "./cmd/tfrelay-source-planfile"); err != nil {
		return fmt.Errorf("build Source command: %w", err)
	}

	environments, err := subdirectories(terraformRoot)
	if err != nil {
		return err
	}
	cases, err := subdirectories(casesRoot)
	if err != nil {
		return err
	}
	for _, environment := range environments {
		versionData, err := os.ReadFile(filepath.Join(terraformRoot, environment, "terraform-version"))
		if err != nil {
			return err
		}
		wantVersion := strings.TrimSpace(string(versionData))
		if terraformVersion != wantVersion {
			return fmt.Errorf("environment %s requires Terraform %s, found %s", environment, wantVersion, terraformVersion)
		}
		for _, testCase := range cases {
			if err := runCase(terraformRoot, casesRoot, snapshotRoot, environment, testCase, sourceBinary, tempRoot, update); err != nil {
				return err
			}
		}
	}
	return nil
}

func runCase(environmentsRoot, casesRoot, snapshotsRoot, environment, testCase, sourceBinary, tempRoot string, update bool) error {
	workDir, err := os.MkdirTemp(tempRoot, "case-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	environmentDir := filepath.Join(environmentsRoot, environment)
	caseDir := filepath.Join(casesRoot, testCase)
	if err := copyFile(filepath.Join(environmentDir, "providers.tf"), filepath.Join(workDir, "providers.tf")); err != nil {
		return err
	}
	if err := copyFile(filepath.Join(environmentDir, ".terraform.lock.hcl"), filepath.Join(workDir, ".terraform.lock.hcl")); err != nil {
		return err
	}
	entries, err := os.ReadDir(caseDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !(strings.HasSuffix(entry.Name(), ".tf") || strings.HasSuffix(entry.Name(), ".tf.json")) {
			continue
		}
		if err := copyFile(filepath.Join(caseDir, entry.Name()), filepath.Join(workDir, entry.Name())); err != nil {
			return err
		}
	}
	if err := runCommand(workDir, "terraform", "init", "-backend=false", "-input=false", "-no-color", "-lockfile=readonly"); err != nil {
		return fmt.Errorf("environment=%s case=%s init: %w", environment, testCase, err)
	}
	if err := runImports(workDir, filepath.Join(caseDir, "imports.json")); err != nil {
		return fmt.Errorf("environment=%s case=%s import: %w", environment, testCase, err)
	}
	if err := runCommand(workDir, "terraform", "plan", "-input=false", "-refresh=false", "-lock=false", "-out=plan.tfplan", "-no-color"); err != nil {
		return fmt.Errorf("environment=%s case=%s plan: %w", environment, testCase, err)
	}
	planData, err := outputIn(workDir, "terraform", "show", "-json", "plan.tfplan")
	if err != nil {
		return fmt.Errorf("environment=%s case=%s show -json: %w", environment, testCase, err)
	}
	canonicalPlan, err := golden.CanonicalPlan(planData)
	if err != nil {
		return fmt.Errorf("environment=%s case=%s canonicalize plan: %w", environment, testCase, err)
	}
	relayData, err := outputIn(workDir, sourceBinary, "--", filepath.Join(workDir, "plan.tfplan"))
	if err != nil {
		return fmt.Errorf("environment=%s case=%s Source: %w", environment, testCase, err)
	}
	canonicalRelay, err := golden.CanonicalRelay(relayData)
	if err != nil {
		return fmt.Errorf("environment=%s case=%s canonicalize Source: %w", environment, testCase, err)
	}
	snapshotDir := filepath.Join(snapshotsRoot, environment, testCase)
	if update {
		if err := os.MkdirAll(snapshotDir, 0755); err != nil {
			return err
		}
		if err := copyFile(filepath.Join(workDir, "plan.tfplan"), filepath.Join(snapshotDir, "plan.tfplan")); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(snapshotDir, "plan.json"), canonicalPlan, 0644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(snapshotDir, "expected.json"), canonicalRelay, 0644); err != nil {
			return err
		}
		fmt.Printf("updated %s/%s\n", environment, testCase)
		return nil
	}
	if err := compareSnapshot(filepath.Join(snapshotDir, "plan.json"), canonicalPlan); err != nil {
		return fmt.Errorf("environment=%s case=%s: %w", environment, testCase, err)
	}
	if err := compareSnapshot(filepath.Join(snapshotDir, "expected.json"), canonicalRelay); err != nil {
		return fmt.Errorf("environment=%s case=%s: %w", environment, testCase, err)
	}
	fmt.Printf("verified %s/%s\n", environment, testCase)
	return nil
}

func runImports(workDir, importFile string) error {
	data, err := os.ReadFile(importFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var imports []importSpec
	if err := json.Unmarshal(data, &imports); err != nil {
		return err
	}
	for _, item := range imports {
		if err := runCommand(workDir, "terraform", "import", "-input=false", "-lock=false", "-no-color", item.Address, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func terraformVersion() (string, error) {
	data, err := exec.Command("terraform", "version", "-json").Output()
	if err != nil {
		return "", fmt.Errorf("terraform version -json: %w", err)
	}
	var info struct {
		Version string `json:"terraform_version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", err
	}
	if info.Version == "" {
		return "", fmt.Errorf("terraform version -json omitted terraform_version")
	}
	return info.Version, nil
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

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0644)
}

func compareSnapshot(path string, actual []byte) error {
	expected, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("snapshot differs at %s; review the change and run `go run ./cmd/golden -update` only to approve an intentional refresh", path)
	}
	return nil
}

func runCommand(dir, name string, args ...string) error {
	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{name}, args...), " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

func outputIn(dir string, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", strings.Join(append([]string{name}, args...), " "), err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
