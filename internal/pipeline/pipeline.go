package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/kamichidu/go-tfrelay"
	"github.com/kamichidu/go-tfrelay/internal/config"
)

type Pipeline struct {
	Config *config.RelayConfig

	ErrWriter io.Writer
}

func (p *Pipeline) doSource(ctx context.Context) (io.Reader, error) {
	fn := func(cfg config.RelayPluginConfig) (json.RawMessage, error) {
		var stdout bytes.Buffer
		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		cmd.Dir = cfg.Dir
		cmd.Stdout = &stdout
		cmd.Stderr = p.ErrWriter
		cmd.Env = cfg.GetEnv()
		log.Printf("info: spawn %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		log.Printf("info: success %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
		if !json.Valid(stdout.Bytes()) {
			log.Printf("error: invalid stdout: not a valid json")
			return nil, fmt.Errorf("invalid stdout: %s", cmd.String())
		}
		return json.RawMessage(stdout.Bytes()), nil
	}

	for _, cfg := range p.Config.Plugins.Source {
		b, err := fn(cfg)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(b), nil
	}
	return nil, errors.New("no sources")
}

func (p *Pipeline) doSink(ctx context.Context) (io.WriteCloser, <-chan struct{}, error) {
	var wg sync.WaitGroup
	fn := func(cfg config.RelayPluginConfig, r io.Reader) {
		defer wg.Done()

		cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
		cmd.Dir = cfg.Dir
		cmd.Stdin = r
		cmd.Stdout = os.Stdout
		cmd.Stderr = p.ErrWriter
		cmd.Env = cfg.GetEnv()
		log.Printf("info: spawn %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
		if err := cmd.Run(); err != nil {
			log.Printf("error: failure %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
			return
		}
		log.Printf("info: success %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
	}

	var (
		writers = make([]io.Writer, 0, len(p.Config.Plugins.Sink))
		closers = make([]io.Closer, 0, len(p.Config.Plugins.Sink))
	)
	for _, cfg := range p.Config.Plugins.Sink {
		wg.Add(1)
		pr, pw := io.Pipe()
		go fn(cfg, pr)
		writers = append(writers, pw)
		closers = append(closers, pw)
	}
	doneCh := make(chan struct{}, 0)
	go func() {
		defer close(doneCh)
		wg.Wait()
	}()
	return &sinkWriter{
		Writer:  io.MultiWriter(writers...),
		Closers: closers,
	}, doneCh, nil
}

type sinkWriter struct {
	io.Writer

	Closers []io.Closer
}

func (w *sinkWriter) Close() error {
	var err error
	for _, c := range w.Closers {
		if e := c.Close(); e != nil {
			err = multierror.Append(err, e)
		}
	}
	return err
}

func (p *Pipeline) Do(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	source, err := p.doSource(ctx)
	if err != nil {
		return err
	}
	sink, sinkDone, err := p.doSink(ctx)
	if err != nil {
		return err
	}
	defer sink.Close()

	var data tfrelay.RelayData
	jd := json.NewDecoder(source)
	if err := jd.Decode(&data); err != nil {
		return err
	}
	je := json.NewEncoder(sink)
	if err := je.Encode(&data); err != nil {
		return err
	}
	if err := sink.Close(); err != nil {
		return err
	}
	select {
	case <-sinkDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
