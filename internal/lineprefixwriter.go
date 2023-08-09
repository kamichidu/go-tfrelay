package internal

import (
	"bytes"
	"io"
)

type LinePrefixWriter struct {
	Prefix string

	Writer io.Writer

	pfx func() error
}

var (
	_ io.Writer = (*LinePrefixWriter)(nil)

	lfBytes = []byte("\n")
)

func (w *LinePrefixWriter) Write(p []byte) (int, error) {
	if w.pfx == nil {
		w.pfx = w.writePrefix
	}
	buffer := p
	for len(buffer) > 0 {
		if err := w.pfx(); err != nil {
			return 0, err
		}

		head, tail, found := bytes.Cut(buffer, lfBytes)
		if !found {
			if _, err := w.Writer.Write(head); err != nil {
				return 0, err
			}
			w.pfx = w.nop
			return len(p), nil
		}

		if _, err := w.Writer.Write(append(head, lfBytes...)); err != nil {
			return 0, err
		}
		w.pfx = w.writePrefix
		buffer = tail
	}
	return len(p), nil
}

func (w *LinePrefixWriter) writePrefix() error {
	_, err := w.Writer.Write([]byte(w.Prefix))
	return err
}

func (w *LinePrefixWriter) nop() error {
	return nil
}
