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
