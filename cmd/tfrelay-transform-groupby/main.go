package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"comail.io/go/colog"
	"github.com/kamichidu/go-flagshim"
	"github.com/kamichidu/go-tfrelay"
)

type command struct{}

func (*command) Name() string { return "tfrelay-transform-groupby" }
func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	return ctx
}
func (c *command) Run(ctx context.Context, _ []string) error {
	var data tfrelay.RelayData
	if err := json.NewDecoder(flagshim.MustStdinFromContext(ctx)).Decode(&data); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	if data.IRVersion != 1 || (data.SnapshotKind != "planned" && data.SnapshotKind != "state") {
		return fmt.Errorf("unsupported RelayData contract: ir_version=%d snapshot_kind=%q", data.IRVersion, data.SnapshotKind)
	}

	data.Resources = groupResources(data.Resources)
	if err := json.NewEncoder(flagshim.MustStdoutFromContext(ctx)).Encode(&data); err != nil {
		return err
	}
	return nil
}

// groupResources merges only equal type/provider pairs, retaining instance
// identity and stable first-seen ordering.
func groupResources(resources []tfrelay.Resource) []tfrelay.Resource {
	indices := map[string]int{}
	grouped := make([]tfrelay.Resource, 0, len(resources))
	for _, resource := range resources {
		key := resource.Type + "\x00" + resource.Provider
		index, ok := indices[key]
		if !ok {
			indices[key] = len(grouped)
			grouped = append(grouped, resource)
			continue
		}
		grouped[index].Name += "/" + resource.Name
		grouped[index].Instances = append(grouped[index].Instances, resource.Instances...)
	}
	return grouped
}

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-transform-groupby ] ")
	os.Exit(flagshim.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, &command{}, os.Args))
}
