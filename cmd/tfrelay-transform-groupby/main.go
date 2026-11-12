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

	// group by resource.type
	memo := map[string]tfrelay.Resource{}
	for _, v := range data.Resources {
		dup, ok := memo[v.Type]
		if ok {
			v.Name = v.Name + "/" + dup.Name
			v.Instances = append(v.Instances, dup.Instances...)
		}
		memo[v.Type] = v
	}
	data.Resources = make([]tfrelay.Resource, 0, len(memo))
	for _, v := range memo {
		data.Resources = append(data.Resources, v)
	}
	if err := json.NewEncoder(flagshim.MustStdoutFromContext(ctx)).Encode(&data); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-transform-groupby ] ")
	os.Exit(flagshim.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, &command{}, os.Args))
}
