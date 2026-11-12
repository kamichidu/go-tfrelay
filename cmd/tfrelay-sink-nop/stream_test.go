package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kamichidu/go-flagshim"
)

func TestCLIStreams(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := flagshim.Run(context.Background(), strings.NewReader(`{"ok":true}`), &stdout, &stderr, &command{}, []string{"cmd"})
	if code != 0 || stdout.String() != "{\"ok\":true}\n" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}
