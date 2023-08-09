package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"comail.io/go/colog"
	"github.com/kamichidu/go-tfrelay"
	"github.com/kamichidu/gobst/funcs"
)

func getenvDefault(key, default_ string) string {
	v := os.Getenv(key)
	if v == "" {
		return default_
	}
	return v
}

func unionKeys(l []tfrelay.Instance) ([]string, error) {
	memo := map[string]struct{}{}
	for i := range l {
		for k := range l[i].Attributes {
			memo[k] = struct{}{}
		}
	}
	keys := make([]string, 0, len(memo))
	for k := range memo {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys, nil
}

func extractColumns(v tfrelay.Instance, keys_ any) ([]string, error) {
	var keys []string
	switch v := keys_.(type) {
	case string:
		keys = []string{v}
	case []string:
		keys = v
	default:
		log.Panicf("invalid argument type: %T", keys_)
	}

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := v.Attributes[k]
		out = append(out, string(v))
	}
	return out, nil
}

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

func newWriteCloser(name string) (io.WriteCloser, error) {
	if name == "-" {
		return &nopCloser{
			Writer: os.Stdout,
		}, nil
	}

	dir := filepath.Dir(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to mkdir: %s: %w", dir, err)
	}
	file, err := os.Create(name)
	if err != nil {
		return nil, fmt.Errorf("failed to create: %s: %w", name, err)
	}
	return file, nil
}

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-sink-text ] ")

	var (
		outFile     = getenvDefault("TFRELAY_SINK_TEXT_OUTPUT", "-")
		templateStr = getenvDefault("TFRELAY_SINK_TEXT_TEMPLATE", "")
	)
	flag.StringVar(&outFile, "out", outFile, "")
	flag.Parse()

	wc, err := newWriteCloser(outFile)
	if err != nil {
		log.Panic(err)
	}
	defer wc.Close()

	var data tfrelay.RelayData
	jd := json.NewDecoder(os.Stdin)
	if err := jd.Decode(&data); err != nil {
		log.Panic(err)
	}

	tpl := template.New("tfrelay-sink-text")
	funcs.Register("unionKeys", unionKeys)
	funcs.Register("extractColumns", extractColumns)
	funcs.Register("evalTemplate", func(name string, arg any) (string, error) {
		v := tpl.Lookup(name)
		if v == nil {
			v = tpl.Lookup("__not_found__")
		}
		if v == nil {
			return "", fmt.Errorf("no template found: %q", name)
		}
		var buffer bytes.Buffer
		if err := v.Execute(&buffer, arg); err != nil {
			return "", err
		}
		return buffer.String(), nil
	})
	tpl = tpl.Funcs(funcs.Funcs())
	tpl = template.Must(tpl.Parse(templateStr))
	if err := tpl.Execute(wc, data); err != nil {
		log.Panic(err)
	}
}
