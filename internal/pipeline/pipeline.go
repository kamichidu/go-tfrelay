package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/kamichidu/go-tfrelay"
	"github.com/kamichidu/go-tfrelay/internal/config"
)

type Pipeline struct {
	Config *config.RelayConfig

	ErrWriter io.Writer
}

func (p *Pipeline) Do(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var r io.Reader = strings.NewReader(`{}`)
	for _, cfg := range p.Config.Plugins.Source {
		r = p.mergeReader(r, cfg)
	}
	for _, cfg := range p.Config.Plugins.Transform {
		r = p.wrapReader(r, cfg)
	}

	sink, sinkDone, err := p.doSink(ctx)
	if err != nil {
		return err
	}
	defer sink.Close()

	var data tfrelay.RelayData
	if err := decodeJSON(r, &data); err != nil {
		return err
	}
	if data.IRVersion != 2 {
		return fmt.Errorf("unsupported RelayData ir_version %d", data.IRVersion)
	}
	if data.SnapshotKind != "planned" && data.SnapshotKind != "state" {
		return fmt.Errorf("unsupported RelayData snapshot_kind %q", data.SnapshotKind)
	}
	if err := encodeJSON(sink, &data); err != nil {
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

func decodeJSON(r io.Reader, v any) error {
	jd := json.NewDecoder(r)
	return jd.Decode(v)
}

func encodeJSON(w io.Writer, v any) error {
	je := json.NewEncoder(w)
	return je.Encode(v)
}
