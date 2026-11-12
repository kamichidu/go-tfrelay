package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"

	"comail.io/go/colog"
	"github.com/kamichidu/go-flagshim"
	"github.com/kamichidu/go-tfrelay/internal/config"
	"github.com/kamichidu/go-tfrelay/internal/pipeline"
)

const progName = "terraform-relay"

//go:embed usage.txt
var usageStr string

type command struct {
	configFile string
}

func (*command) Name() string { return progName }

func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	fs.StringVar(&c.configFile, "config", "", "The configuration file path.")
	fs.Alias("config", "c")
	return ctx
}

func (c *command) Run(ctx context.Context, _ []string) error {
	var loadConfig func() (*config.RelayConfig, error)
	if c.configFile != "" {
		loadConfig = func() (*config.RelayConfig, error) {
			return config.LoadRelayConfig(c.configFile)
		}
	} else {
		loadConfig = func() (*config.RelayConfig, error) {
			return config.LoadRelayConfigFromDir(".")
		}
	}
	cfg, err := loadConfig()
	if err != nil {
		return fmt.Errorf("invalid config file: %w", err)
	}

	log.Printf("debug: timeout %s", cfg.TimeoutDuration)
	ctx, cancel := context.WithTimeout(ctx, cfg.TimeoutDuration)
	defer cancel()

	pl := &pipeline.Pipeline{Config: cfg, ErrWriter: flagshim.MustStderrFromContext(ctx)}
	if err := pl.Do(ctx); err != nil {
		return fmt.Errorf("pipeline execution failed: %w", err)
	}
	return nil
}

type usageCommand struct{ command }

func (c *usageCommand) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	ctx = c.command.RegisterFlags(ctx, fs)
	fs.Usage = func() { fmt.Fprint(fs.Output(), usageStr) }
	return ctx
}

func run(stdout, stderr io.Writer, args []string) int {
	colog.Register()
	colog.SetOutput(stderr)
	colog.SetPrefix("[ " + progName + " ] ")
	return flagshim.Run(context.Background(), os.Stdin, stdout, stderr, &usageCommand{}, args)
}

func main() { os.Exit(run(os.Stdout, os.Stderr, os.Args)) }
