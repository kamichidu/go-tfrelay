package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kamichidu/go-flagshim"
)

func TestCLIHelpAndParseError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, &command{}, []string{"tfrelay-source-planfile", "-h"})
	if code != 0 || !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
		t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, &command{}, []string{"tfrelay-source-planfile", "--unknown"})
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("parse error: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestCLIMissingPlanArgument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(&stdout, &stderr, []string{"tfrelay-source-planfile"})
	if code != 1 || !strings.Contains(stderr.String(), "missing planfile") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestTerraformPassthroughBoundary(t *testing.T) {
	// flagshim, like the standard flag package, stops parsing at the first positional arg.
	var got []string
	c := captureCommand{capture: func(args []string) { got = append([]string(nil), args...) }}
	var stdout, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, c, []string{"cmd", "saved.plan", "-var", "x=y", "--", "-target=resource.x"})
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if strings.Join(got, "|") != "saved.plan|-var|x=y|--|-target=resource.x" {
		t.Fatalf("args=%q", got)
	}
}

type captureCommand struct{ capture func([]string) }

func (captureCommand) Name() string { return "capture" }
func (captureCommand) RegisterFlags(ctx context.Context, _ *flagshim.FlagSet) context.Context {
	return ctx
}
func (c captureCommand) Run(_ context.Context, args []string) error { c.capture(args); return nil }
