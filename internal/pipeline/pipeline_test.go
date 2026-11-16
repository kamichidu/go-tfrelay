package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kamichidu/go-tfrelay/internal/config"
)

func TestPipelineAcceptsFirstSourceSnapshotKind(t *testing.T) {
	for _, kind := range []string{"planned", "state"} {
		t.Run(kind, func(t *testing.T) {
			got := runPipeline(t, []config.RelayPluginConfig{sourceConfig(`{"ir_version":1,"snapshot_kind":"` + kind + `","resources":[]}`)})
			if !strings.Contains(got, `"snapshot_kind":"`+kind+`"`) || !strings.Contains(got, `"ir_version":1`) {
				t.Fatalf("pipeline output did not preserve v1 %s source: %s", kind, got)
			}
		})
	}
}

func TestPipelineRejectsMixedSourceSnapshotKinds(t *testing.T) {
	first := sourceConfig(`{"ir_version":1,"snapshot_kind":"planned","resources":[]}`)
	second := sourceConfig(`{"ir_version":1,"snapshot_kind":"state","resources":[]}`)
	output := filepath.Join(t.TempDir(), "sink.json")
	p := &Pipeline{Config: &config.RelayConfig{Plugins: config.RelayPluginsConfig{
		Source: []config.RelayPluginConfig{first, second},
		Sink:   []config.RelayPluginConfig{{Command: "sh", Args: []string{"-c", "cat > " + output}}},
	}}}
	if err := p.Do(context.Background()); err == nil || !strings.Contains(err.Error(), "different RelayData contracts") {
		t.Fatalf("mixed source kinds should fail, got %v", err)
	}
}

func TestPipelineZeroSourcesProduceEmptyStateSnapshot(t *testing.T) {
	got := runPipeline(t, nil)
	if got != `{"ir_version":1,"snapshot_kind":"state","resources":[]}` {
		t.Fatalf("zero-source output = %s", got)
	}
}

func TestPipelineReturnsSinkChildFailure(t *testing.T) {
	command := "cat >/dev/null; exit 23"
	err := runPipelineWithSinks(t, []config.RelayPluginConfig{{Command: "sh", Args: []string{"-c", command}}})
	if err == nil {
		t.Fatal("expected sink child failure")
	}
	if !strings.Contains(err.Error(), "sink /usr/bin/sh -c "+command) {
		t.Fatalf("sink failure should identify command, got %v", err)
	}
}

func TestPipelineReturnsSinkFailureWhenAnotherSinkSucceeds(t *testing.T) {
	successOutput := filepath.Join(t.TempDir(), "success.json")
	command := "cat >/dev/null; exit 23"
	err := runPipelineWithSinks(t, []config.RelayPluginConfig{
		{Command: "sh", Args: []string{"-c", "cat > " + successOutput}},
		{Command: "sh", Args: []string{"-c", command}},
	})
	if err == nil {
		t.Fatal("expected pipeline error when one sink fails")
	}
	if data, readErr := os.ReadFile(successOutput); readErr != nil || !strings.Contains(string(data), `"snapshot_kind":"state"`) {
		t.Fatalf("successful sink did not receive snapshot: data=%q err=%v", data, readErr)
	}
}

func TestPipelineReturnsAllSinkFailures(t *testing.T) {
	commands := []string{"cat >/dev/null; exit 23", "cat >/dev/null; exit 29"}
	sinks := make([]config.RelayPluginConfig, 0, len(commands))
	for _, command := range commands {
		sinks = append(sinks, config.RelayPluginConfig{Command: "sh", Args: []string{"-c", command}})
	}
	err := runPipelineWithSinks(t, sinks)
	if err == nil {
		t.Fatal("expected pipeline error when all sinks fail")
	}
	for _, command := range commands {
		if !strings.Contains(err.Error(), "sink /usr/bin/sh -c "+command) {
			t.Errorf("error %q does not identify failing command %q", err, command)
		}
	}
}

func runPipelineWithSinks(t *testing.T, sinks []config.RelayPluginConfig) error {
	t.Helper()
	p := &Pipeline{Config: &config.RelayConfig{Plugins: config.RelayPluginsConfig{Sink: sinks}}}
	return p.Do(context.Background())
}

func runPipeline(t *testing.T, sources []config.RelayPluginConfig) string {
	t.Helper()
	output := filepath.Join(t.TempDir(), "sink.json")
	p := &Pipeline{Config: &config.RelayConfig{Plugins: config.RelayPluginsConfig{
		Source: sources,
		Sink:   []config.RelayPluginConfig{{Command: "sh", Args: []string{"-c", "cat > " + output}}},
	}}}
	if err := p.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

func sourceConfig(data string) config.RelayPluginConfig {
	return config.RelayPluginConfig{Command: "sh", Args: []string{"-c", "printf '%s' '" + data + "'"}}
}
