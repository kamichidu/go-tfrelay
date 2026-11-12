package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"comail.io/go/colog"
	"github.com/kamichidu/go-flagshim"
)

type command struct {
	pretty bool
}

func (*command) Name() string { return "tfrelay-sink-nop" }
func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	fs.BoolVar(&c.pretty, "pretty", false, "")
	return ctx
}
func (c *command) Run(ctx context.Context, _ []string) error {
	jd := json.NewDecoder(flagshim.MustStdinFromContext(ctx))
	je := json.NewEncoder(flagshim.MustStdoutFromContext(ctx))
	if c.pretty {
		je.SetIndent("", "  ")
	}
	var data json.RawMessage
	if err := jd.Decode(&data); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	if err := je.Encode(data); err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	return nil
}

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-sink-nop ] ")
	os.Exit(flagshim.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, &command{}, os.Args))
}
