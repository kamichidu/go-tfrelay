package main

import (
	"context"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"comail.io/go/colog"
	"github.com/kamichidu/go-tfrelay/internal/config"
	"github.com/kamichidu/go-tfrelay/internal/pipeline"
)

const (
	progName = "terraform-relay"
)

var (
	//go:embed usage.txt
	usageStr string
)

func run(stdout, stderr io.Writer, args []string) int {
	colog.Register()
	colog.SetOutput(stderr)
	colog.SetPrefix("[ " + progName + " ] ")

	var (
		showHelp   bool
		configFile string
	)
	flgs := flag.NewFlagSet(progName, flag.ContinueOnError)
	flgs.SetOutput(io.Discard)
	flgs.BoolVar(&showHelp, "help", showHelp, "")
	flgs.StringVar(&configFile, "config", configFile, "")
	switch err := flgs.Parse(args[1:]); {
	case errors.Is(err, flag.ErrHelp) || showHelp:
		fmt.Fprintln(stdout, usageStr)
		return 0
	case err != nil:
		log.Printf("error: invalid cli argument: %v", err)
		fmt.Fprintln(stderr, usageStr)
		return 2
	}

	var loadConfig func() (*config.RelayConfig, error)
	if configFile != "" {
		loadConfig = func() (*config.RelayConfig, error) {
			return config.LoadRelayConfig(configFile)
		}
	} else {
		loadConfig = func() (*config.RelayConfig, error) {
			return config.LoadRelayConfigFromDir(".")
		}
	}
	cfg, err := loadConfig()
	if err != nil {
		log.Printf("error: invalid config file: %v", err)
		return 1
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pl := &pipeline.Pipeline{
		Config:    cfg,
		ErrWriter: os.Stderr,
	}
	if err := pl.Do(ctx); err != nil {
		log.Printf("error: pipeline execution failed: %v", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args))
}
