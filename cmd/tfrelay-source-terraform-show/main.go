package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"comail.io/go/colog"
	"github.com/kamichidu/go-flagshim"
	"github.com/kamichidu/go-tfrelay"
	"github.com/kamichidu/go-tfrelay/internal/showjson"
)

const progName = "tfrelay-source-terraform-show"

//go:embed usage.txt
var usageStr string

type command struct {
	plan      bool
	planfile  string
	statefile string

	lookPath       func(string) (string, error)
	commandFactory func(context.Context, string, ...string) *exec.Cmd
}

func (*command) Name() string { return progName }
func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	fs.Usage = func() { fmt.Fprint(fs.Output(), usageStr) }
	fs.BoolVar(&c.plan, "plan", false, "run terraform plan before showing the plan")
	fs.StringVar(&c.planfile, "planfile", "", "plan file to show or create")
	fs.StringVar(&c.statefile, "statefile", "", "state file to show")
	return ctx
}

func (c *command) Run(ctx context.Context, args []string) error {
	if c.planfile != "" && c.statefile != "" {
		return errors.New("-planfile and -statefile are mutually exclusive")
	}
	if c.plan && c.planfile == "" {
		return errors.New("-plan requires -planfile")
	}
	if len(args) != 0 && !c.plan {
		return errors.New("Terraform plan passthrough requires -plan")
	}
	if !c.plan && len(args) != 0 {
		return errors.New("unexpected arguments")
	}
	if c.plan && hasOutArgument(args) {
		return errors.New("Terraform plan passthrough must not specify -out; use -planfile")
	}

	terraform, err := c.findTerraform()
	if err != nil {
		return err
	}
	stdout, stderr := flagshim.MustStdoutFromContext(ctx), flagshim.MustStderrFromContext(ctx)
	if c.plan {
		planArgs := []string{"plan", "-out=" + c.planfile}
		planArgs = append(planArgs, args...)
		cmd := c.newCommand(ctx, terraform, planArgs...)
		cmd.Stdin = flagshim.MustStdinFromContext(ctx)
		cmd.Stdout, cmd.Stderr = stderr, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("terraform plan: %w", err)
		}
		return c.show(ctx, terraform, c.planfile, true, stdout, stderr)
	}
	if c.statefile != "" {
		return c.show(ctx, terraform, c.statefile, false, stdout, stderr)
	}
	if c.planfile != "" {
		return c.show(ctx, terraform, c.planfile, true, stdout, stderr)
	}
	return c.show(ctx, terraform, "", false, stdout, stderr)
}

func (c *command) findTerraform() (string, error) {
	lookup := c.lookPath
	if lookup == nil {
		lookup = exec.LookPath
	}
	path, err := lookup("terraform")
	if err != nil {
		return "", fmt.Errorf("find terraform: %w", err)
	}
	return path, nil
}

func (c *command) newCommand(ctx context.Context, path string, args ...string) *exec.Cmd {
	if c.commandFactory != nil {
		return c.commandFactory(ctx, path, args...)
	}
	return exec.CommandContext(ctx, path, args...)
}

func (c *command) show(ctx context.Context, terraform, file string, planned bool, stdout, stderr io.Writer) error {
	args := []string{"show", "-json"}
	if file != "" {
		args = append(args, file)
	}
	cmd := c.newCommand(ctx, terraform, args...)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("terraform show -json: %w", err)
	}
	data, err := decodeSnapshot(output.Bytes(), planned)
	if err != nil {
		return fmt.Errorf("decode terraform show -json: %w", err)
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("write RelayData: %w", err)
	}
	return nil
}

func hasOutArgument(args []string) bool {
	for _, arg := range args {
		if arg == "-out" || strings.HasPrefix(arg, "-out=") || strings.HasPrefix(arg, "-out ") {
			return true
		}
	}
	return false
}

func (*command) MapError(err error) int {
	var parseErr *flagshim.ParseError
	if errors.As(err, &parseErr) && errors.Is(parseErr.Err, flag.ErrHelp) {
		return 0
	}
	if errors.As(err, &parseErr) {
		return 2
	}
	return 1
}

func decodeSnapshot(input []byte, planned bool) (tfrelay.RelayData, error) {
	return showjson.DecodeSnapshot(input, planned)
}

func run(stdout, stderr io.Writer, args []string) int {
	colog.Register()
	colog.SetOutput(stderr)
	colog.SetPrefix("[ " + progName + " ] ")
	return flagshim.Run(context.Background(), os.Stdin, stdout, stderr, &command{}, args)
}

func main() { os.Exit(run(os.Stdout, os.Stderr, os.Args)) }
