package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kamichidu/go-flagshim"
)

func TestCLIStreams(t *testing.T) {
	t.Setenv("TFRELAY_SINK_TEXT_OUTPUT", "-")
	t.Setenv("TFRELAY_SINK_TEXT_TEMPLATE", "ok")
	c := &command{}
	var stdout, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(`{"resources":[]}`), &stdout, &stderr, c, []string{"cmd"})
	if code != 0 || stdout.String() != "ok" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
