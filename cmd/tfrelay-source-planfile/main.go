package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"comail.io/go/colog"
	"github.com/google/uuid"
	"github.com/kamichidu/go-flagshim"
	"github.com/kamichidu/go-tfrelay"
)

const progName = "tfrelay-source-planfile"

//go:embed usage.txt
var usageStr string

type command struct{}

func (*command) Name() string { return progName }
func (c *command) RegisterFlags(ctx context.Context, fs *flagshim.FlagSet) context.Context {
	fs.Usage = func() { fmt.Fprint(fs.Output(), usageStr) }
	return ctx
}

func toInstance(src tfInstance) tfrelay.Instance {
	return tfrelay.Instance{IndexKey: src.IndexKeyString(), Attributes: src.Attributes, SensitiveAttributes: src.SensitiveAttributes}
}

func toResource(src tfResource) tfrelay.Resource {
	out := tfrelay.Resource{ResourceID: uuid.NewString(), Provider: src.Provider, Type: src.Type, Name: src.Name, Instances: make([]tfrelay.Instance, len(src.Instances))}
	for i := range src.Instances {
		out.Instances[i] = toInstance(src.Instances[i])
	}
	return out
}

func toRelayData(src tfState) tfrelay.RelayData {
	out := tfrelay.RelayData{Resources: make([]tfrelay.Resource, len(src.Resources))}
	for i := range src.Resources {
		out.Resources[i] = toResource(src.Resources[i])
	}
	return out
}

func (c *command) Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("missing planfile or Terraform plan command")
	}
	planArg := args[0]
	terraformArgs := args[1:]
	var planfile string
	if _, err := os.Stat(planArg); errors.Is(err, os.ErrNotExist) {
		v, err := createPlanfile(planArg, terraformArgs...)
		if err != nil {
			return fmt.Errorf("failed to command %v: %w", args, err)
		}
		planfile = v
	} else if err != nil {
		return fmt.Errorf("io error: %w", err)
	} else {
		if len(terraformArgs) != 0 {
			return fmt.Errorf("unexpected arguments after planfile %q: %v", planArg, terraformArgs)
		}
		planfile = planArg
	}

	log.Printf("info: open planfile: %v", planfile)
	pf, err := openPlanfile(planfile)
	if err != nil {
		return fmt.Errorf("failed to open planfile: %w", err)
	}
	defer pf.Close()

	plan, err := pf.GetTfPlan()
	if err != nil {
		return fmt.Errorf("failed to get tfplan from planfile: %w", err)
	}
	state, err := pf.GetTfState()
	if err != nil {
		return fmt.Errorf("failed to get tfstate from planfile: %w", err)
	}

	log.Print("debug: variables:")
	for k, dv := range plan.Variables {
		val := mustDecodeMsgpack(dv.Msgpack)
		log.Printf("debug: - %v - %v\n", k, val.GoString())
	}
	log.Print("debug: resourceChanges:")
	for _, rc := range plan.ResourceChanges {
		state.Apply(rc)
	}
	je := json.NewEncoder(flagshim.MustStdoutFromContext(ctx))
	je.SetIndent("", "  ")
	if err := je.Encode(toRelayData(state)); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
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

func run(stdout, stderr io.Writer, args []string) int {
	colog.Register()
	colog.SetOutput(stderr)
	colog.SetPrefix("[ " + progName + " ] ")
	return flagshim.Run(context.Background(), os.Stdin, stdout, stderr, &command{}, args)
}

func main() { os.Exit(run(os.Stdout, os.Stderr, os.Args)) }
