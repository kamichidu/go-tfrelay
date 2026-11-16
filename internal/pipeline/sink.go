package pipeline

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-multierror"
	"github.com/kamichidu/go-tfrelay/internal/config"
)

func (p *Pipeline) doSink(ctx context.Context) (io.WriteCloser, <-chan error, error) {
	var wg sync.WaitGroup
	var failures error
	var failuresMu sync.Mutex
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
			failuresMu.Lock()
			failures = multierror.Append(failures, fmt.Errorf("sink %s: %w", cmd.String(), err))
			failuresMu.Unlock()
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
	doneCh := make(chan error, 1)
	go func() {
		wg.Wait()
		doneCh <- failures
		close(doneCh)
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
