package pipeline

import (
	"bytes"
	"io"
	"log"
	"os/exec"

	"github.com/kamichidu/go-tfrelay"
	"github.com/kamichidu/go-tfrelay/internal/config"
)

func (p *Pipeline) mergeReader(r io.Reader, cfg config.RelayPluginConfig) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()

		var x tfrelay.RelayData
		if err := decodeJSON(r, &x); err != nil {
			log.Printf("error: failed to merge: %v", err)
			pw.CloseWithError(err)
			return
		}

		var stdout bytes.Buffer
		cmd := exec.Command(cfg.Command, cfg.Args...)
		cmd.Dir = cfg.Dir
		cmd.Env = cfg.GetEnv()
		cmd.Stdin = nil
		cmd.Stdout = &stdout
		cmd.Stderr = p.ErrWriter
		log.Printf("info: spawn %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
		if err := cmd.Run(); err != nil {
			log.Printf("error: failure %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
			pw.CloseWithError(err)
			return
		}
		log.Printf("info: success %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)

		var y tfrelay.RelayData
		if err := decodeJSON(&stdout, &y); err != nil {
			log.Printf("error: failed to merge: %v", err)
			pw.CloseWithError(err)
			return
		}

		x.Resources = append(x.Resources, y.Resources...)
		if err := encodeJSON(pw, &x); err != nil {
			log.Printf("error: failed to write merged data: %v", err)
			pw.CloseWithError(err)
			return
		}
	}()
	return pr
}

func (p *Pipeline) wrapReader(r io.Reader, cfg config.RelayPluginConfig) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()

		cmd := exec.Command(cfg.Command, cfg.Args...)
		cmd.Dir = cfg.Dir
		cmd.Env = cfg.GetEnv()
		cmd.Stdin = r
		cmd.Stdout = pw
		cmd.Stderr = p.ErrWriter
		log.Printf("info: spawn %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
		if err := cmd.Run(); err != nil {
			log.Printf("error: failure %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
			pw.CloseWithError(err)
			return
		}
		log.Printf("info: success %s\n\tworking_dir=%s", cmd.String(), cmd.Dir)
	}()
	return pr
}
