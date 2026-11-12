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
	code := flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, &command{}, []string{"tfrelay-transform-groupby", "-h"})
	if code != 0 || !strings.Contains(stdout.String(), "Usage of tfrelay-transform-groupby") || stderr.Len() != 0 {
		t.Fatalf("help: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = flagshim.Run(context.Background(), strings.NewReader(""), &stdout, &stderr, &command{}, []string{"tfrelay-transform-groupby", "--unknown"})
	if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "Usage of tfrelay-transform-groupby") {
		t.Fatalf("parse error: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
