package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/kamichidu/go-flagshim"
)

func TestPlanMappingRecursiveUnknownSensitiveAndIdentity(t *testing.T) {
	input := []byte(`{
      "planned_values":{"root_module":{"resources":[
        {"address":"aws_x.item[0]","type":"aws_x","name":"item","provider_name":"registry/x","index":0,"values":{"nested":{"known":1,"nil":null},"secret":{"inside":"s"},"items":[1,null] },"sensitive_values":{"secret":{"inside":true}}}
      ],"child_modules":[{"resources":[
        {"address":"module.child.aws_x.item[\"blue\"]","type":"aws_x","name":"item","provider_name":"registry/x","index":"blue","values":{"nested":{"known":2}}}
      ]}] }},
      "resource_changes":[{"address":"aws_x.item[0]","change":{"after_unknown":{"nested":{"missing":true},"items":[false,true],"whole_missing":{"leaf":true}},"after_sensitive":{"nested":{"known":true}}}}]
    }`)
	got, err := decodeSnapshot(input, true)
	if err != nil {
		t.Fatal(err)
	}
	if got.IRVersion != 2 || got.SnapshotKind != "planned" || len(got.Resources) != 2 {
		t.Fatalf("snapshot metadata/resources: %+v", got)
	}
	var attrs map[string]json.RawMessage
	for _, resource := range got.Resources {
		for _, instance := range resource.Instances {
			if instance.Address == "aws_x.item[0]" {
				b, _ := json.Marshal(instance.Attributes)
				_ = json.Unmarshal(b, &attrs)
			}
		}
	}
	if attrs == nil {
		t.Fatal("counted instance address missing")
	}
	var nestedValue map[string]json.RawMessage
	if err := json.Unmarshal(attrs["nested"], &nestedValue); err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(nestedValue["value"], &values); err != nil {
		t.Fatal(err)
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(values["known"], &known); err != nil {
		t.Fatal(err)
	}
	if string(known["value"]) != `1` {
		t.Fatalf("known number lost precision/value: %s", known["value"])
	}
	if string(values["missing"]) != `{"state":"unknown"}` {
		t.Fatalf("explicit missing unknown path not materialized: %s", values["missing"])
	}
	var wholeMissing map[string]json.RawMessage
	if err := json.Unmarshal(attrs["whole_missing"], &wholeMissing); err != nil {
		t.Fatal(err)
	}
	var wholeChildren map[string]json.RawMessage
	if err := json.Unmarshal(wholeMissing["value"], &wholeChildren); err != nil || string(wholeChildren["leaf"]) != `{"state":"unknown"}` {
		t.Fatalf("nested missing mask path not materialized: %s (%v)", wholeMissing["value"], err)
	}
	if string(known["sensitive"]) != `true` {
		t.Fatalf("nested sensitive mask lost: %s", known["sensitive"])
	}
	var nullValue map[string]any
	if err := json.Unmarshal(values["nil"], &nullValue); err != nil || nullValue["state"] != "known" || nullValue["value"] != nil {
		t.Fatalf("known null lost: %s (%v)", values["nil"], err)
	}
	addressFound := false
	for _, resource := range got.Resources {
		for _, instance := range resource.Instances {
			if instance.Address == "aws_x.item[0]" {
				addressFound = true
			}
		}
	}
	if !addressFound {
		t.Fatal("address missing")
	}
	foundIndex := false
	for _, resource := range got.Resources {
		for _, instance := range resource.Instances {
			if instance.Address == "aws_x.item[0]" && instance.IndexKey == "0" {
				foundIndex = true
			}
		}
	}
	if !foundIndex {
		t.Fatal("numeric index key missing")
	}
}

func TestStateMappingAndPlanEnvelopeValidation(t *testing.T) {
	state := []byte(`{"values":{"root_module":{"resources":[{"address":"aws_x.one","type":"aws_x","name":"one","values":{"empty":null,"n":12345678901234567890},"sensitive_values":{"n":true}}]}}}`)
	got, err := decodeSnapshot(state, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotKind != "state" || got.Resources[0].Instances[0].Attributes["n"].Sensitive != true {
		t.Fatalf("state snapshot incorrect: %+v", got)
	}
	for _, tc := range []struct{ name, field string }{{"errored", `"errored":true`}, {"incomplete", `"complete":false`}, {"deferred", `"deferred_changes":[{}]`}} {
		t.Run(tc.name, func(t *testing.T) {
			input := []byte(`{"planned_values":{"root_module":{"resources":[]}},` + tc.field + `}`)
			if _, err := decodeSnapshot(input, true); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestCLIFlagsAndTerraformInvocations(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-planfile=x", "-statefile=y"}, "mutually exclusive"},
		{[]string{"-plan"}, "requires -planfile"},
		{[]string{"--", "-var=x"}, "requires -plan"},
		{[]string{"-plan", "-planfile=x", "--", "-out=y"}, "must not specify -out"},
	} {
		var out, stderr bytes.Buffer
		code := run(&out, &stderr, append([]string{"cmd"}, tc.args...))
		if code != 1 || !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("args=%v code=%d stderr=%q", tc.args, code, stderr.String())
		}
	}

	fixture := `{"planned_values":{"root_module":{"resources":[]}}}`
	var calls [][]string
	c := &command{lookPath: func(string) (string, error) { return "fake-terraform", nil }}
	c.commandFactory = func(_ context.Context, _ string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 0 && args[0] == "show" {
			return exec.Command("printf", "%s", fixture)
		}
		return exec.Command("true")
	}
	var out, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(""), &out, &stderr, c, []string{"cmd", "-plan", "-planfile=p.tfplan", "--", "-var=a=b"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if len(calls) != 2 || !reflect.DeepEqual(calls[0], []string{"plan", "-out=p.tfplan", "-var=a=b"}) || !reflect.DeepEqual(calls[1], []string{"show", "-json", "p.tfplan"}) {
		t.Fatalf("terraform args=%v", calls)
	}
	if !strings.Contains(out.String(), `"snapshot_kind": "planned"`) {
		t.Fatalf("stdout=%s", out.String())
	}
}

func TestCLIStateAndExistingPlanModes(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args, want []string
		snapshot   string
	}{
		{"current state", nil, []string{"show", "-json"}, `"snapshot_kind": "state"`},
		{"state file", []string{"-statefile=backup.tfstate"}, []string{"show", "-json", "backup.tfstate"}, `"snapshot_kind": "state"`},
		{"existing plan", []string{"-planfile=saved.tfplan"}, []string{"show", "-json", "saved.tfplan"}, `"snapshot_kind": "planned"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := `{"planned_values":{"root_module":{"resources":[]}},"values":{"root_module":{"resources":[]}}}`
			var gotArgs []string
			c := &command{lookPath: func(string) (string, error) { return "fake", nil }, commandFactory: func(_ context.Context, _ string, args ...string) *exec.Cmd {
				gotArgs = append([]string(nil), args...)
				return exec.Command("printf", "%s", fixture)
			}}
			var stdout, stderr bytes.Buffer
			allArgs := append([]string{"cmd"}, tc.args...)
			if code := flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, c, allArgs); code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr.String())
			}
			if !reflect.DeepEqual(gotArgs, tc.want) {
				t.Fatalf("Terraform args=%v want %v", gotArgs, tc.want)
			}
			if !strings.Contains(stdout.String(), tc.snapshot) {
				t.Fatalf("stdout=%s", stdout.String())
			}
		})
	}
}

func TestCLIHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, &command{}, []string{"cmd", "-h"})
	if code != 0 || !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
