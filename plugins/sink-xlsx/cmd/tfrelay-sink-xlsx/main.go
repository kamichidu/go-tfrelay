package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kamichidu/go-tfrelay/plugins/sink-xlsx/internal/projection"
	"github.com/kamichidu/go-tfrelay/plugins/sink-xlsx/internal/workbook"
)

func main() {
	out := flag.String("out", envOr("TFRELAY_SINK_XLSX_OUTPUT", "-"), "output XLSX path, or - for stdout")
	flag.Parse()
	if err := run(os.Stdin, os.Stdout, *out); err != nil {
		fmt.Fprintln(os.Stderr, "tfrelay-sink-xlsx:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run(input io.Reader, stdout io.Writer, output string) error {
	decoder := json.NewDecoder(input)
	var data projection.RelayData
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON data")
		}
		return fmt.Errorf("trailing input: %w", err)
	}
	var workbookBytes bytes.Buffer
	if err := workbook.Write(data, &workbookBytes); err != nil {
		return fmt.Errorf("create workbook: %w", err)
	}
	if output == "-" {
		_, err := stdout.Write(workbookBytes.Bytes())
		return err
	}
	return atomicWrite(output, workbookBytes.Bytes())
}

func atomicWrite(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".tfrelay-sink-xlsx-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err = tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary workbook: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary workbook: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("close temporary workbook: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}
