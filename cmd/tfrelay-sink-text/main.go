package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	"comail.io/go/colog"
	"github.com/kamichidu/go-flagshim"
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
		value := v.Attributes[k]
		if value.State == "unknown" {
			out = append(out, "<unknown>")
			continue
		}
		out = append(out, string(value.Value))
	}
	return out, nil
}

type nopCloser struct {
	io.Writer
}

func (*nopCloser) Close() error {
	return nil
}

func newWriteCloser(name string, stdout io.Writer) (io.WriteCloser, error) {
	if name == "-" {
		return &nopCloser{
			Writer: stdout,
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

type command struct {
	outFile     string
	templateStr string
}

func (*command) Name() string { return "tfrelay-sink-text" }
func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	c.outFile = getenvDefault("TFRELAY_SINK_TEXT_OUTPUT", "-")
	c.templateStr = getenvDefault("TFRELAY_SINK_TEXT_TEMPLATE", "")
	fs.StringVar(&c.outFile, "out", c.outFile, "")
	return ctx
}

func (c *command) Run(ctx context.Context, _ []string) error {

	wc, err := newWriteCloser(c.outFile, flagshim.MustStdoutFromContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = wc.Close() }()

	var data tfrelay.RelayData
	jd := json.NewDecoder(flagshim.MustStdinFromContext(ctx))
	if err := jd.Decode(&data); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	if data.IRVersion != 1 || (data.SnapshotKind != "planned" && data.SnapshotKind != "state") {
		return fmt.Errorf("unsupported RelayData contract: ir_version=%d snapshot_kind=%q", data.IRVersion, data.SnapshotKind)
	}
	for resourceIndex := range data.Resources {
		for instanceIndex := range data.Resources[resourceIndex].Instances {
			instance := &data.Resources[resourceIndex].Instances[instanceIndex]
			for name, value := range instance.Attributes {
				redacted, err := redactValue(value)
				if err != nil {
					return fmt.Errorf("redact %s.%s: %w", instance.Address, name, err)
				}
				instance.Attributes[name] = redacted
			}
		}
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
	tpl = template.Must(tpl.Parse(c.templateStr))
	if err := tpl.Execute(wc, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}
	return nil
}

func redactValue(value tfrelay.Value) (tfrelay.Value, error) {
	if value.Sensitive {
		value.State, value.Type = "known", "string"
		value.Value, _ = json.Marshal("<sensitive>")
		return value, nil
	}
	if value.State != "known" {
		return value, nil
	}
	switch value.Type {
	case "object":
		var children map[string]tfrelay.Value
		if err := json.Unmarshal(value.Value, &children); err != nil {
			return value, err
		}
		for key, child := range children {
			redacted, err := redactValue(child)
			if err != nil {
				return value, err
			}
			children[key] = redacted
		}
		encoded, err := json.Marshal(children)
		if err != nil {
			return value, err
		}
		value.Value = encoded
	case "array":
		var children []tfrelay.Value
		if err := json.Unmarshal(value.Value, &children); err != nil {
			return value, err
		}
		for index, child := range children {
			redacted, err := redactValue(child)
			if err != nil {
				return value, err
			}
			children[index] = redacted
		}
		encoded, err := json.Marshal(children)
		if err != nil {
			return value, err
		}
		value.Value = encoded
	}
	return value, nil
}

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-sink-text ] ")
	os.Exit(flagshim.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, &command{}, os.Args))
}
