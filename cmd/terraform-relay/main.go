package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"comail.io/go/colog"
	"github.com/kamichidu/go-tfrelay/internal/config"
)

func main() {
	colog.Register()
	colog.SetPrefix("[ terraform-relay ] ")

	flag.Parse()

	cfg, err := config.LoadRelayConfig(".")
	if err != nil {
		log.Panic(err)
	}

	pl := &pipeline{
		Config:    cfg,
		ErrWriter: os.Stderr,
	}
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pl.Do(ctx); err != nil {
		log.Panic(err)
	}
}
