package main

import (
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
	"github.com/kamichidu/go-tfrelay"
)

const (
	progName = "tfrelay-source-planfile"
)

var (
	//go:embed usage.txt
	usageStr string
)

func toInstance(src tfInstance) tfrelay.Instance {
	var out tfrelay.Instance
	out.IndexKey = src.IndexKeyString()
	out.Attributes = src.Attributes
	out.SensitiveAttributes = src.SensitiveAttributes
	return out
}

func toResource(src tfResource) tfrelay.Resource {
	var out tfrelay.Resource
	out.ResourceID = uuid.NewString()
	out.Provider = src.Provider
	out.Type = src.Type
	out.Name = src.Name
	out.Instances = make([]tfrelay.Instance, len(src.Instances))
	for i := range src.Instances {
		out.Instances[i] = toInstance(src.Instances[i])
	}
	return out
}

func toRelayData(src tfState) tfrelay.RelayData {
	var out tfrelay.RelayData
	out.Resources = make([]tfrelay.Resource, len(src.Resources))
	for i := range src.Resources {
		out.Resources[i] = toResource(src.Resources[i])
	}
	return out
}

func run(stdout, stderr io.Writer, args []string) int {
	colog.Register()
	colog.SetOutput(stderr)
	colog.SetPrefix("[ tfrelay-source-planfile ] ")

	var (
		showHelp bool
	)
	flgs := flag.NewFlagSet(progName, flag.ContinueOnError)
	flgs.SetOutput(io.Discard)
	flgs.BoolVar(&showHelp, "help", showHelp, "")
	switch err := flgs.Parse(args[1:]); {
	case errors.Is(err, flag.ErrHelp) || showHelp:
		fmt.Fprintln(stdout, usageStr)
		return 0
	case err != nil:
		log.Printf("error: invalid cli argument: %v", err)
		fmt.Fprintln(stderr, usageStr)
		return 2
	}

	var planfile string
	if _, err := os.Stat(flgs.Arg(0)); errors.Is(err, os.ErrNotExist) {
		v, err := createPlanfile(flgs.Arg(0), flgs.Args()[1:]...)
		if err != nil {
			log.Printf("error: failed to command %v: %v", flgs.Args(), err)
			return 1
		}
		planfile = v
	} else if err != nil {
		log.Printf("error: io error: %v", err)
		return 1
	} else {
		planfile = flgs.Arg(0)
	}

	log.Printf("info: open planfile: %v", planfile)
	pf, err := openPlanfile(planfile)
	if err != nil {
		log.Printf("error: failed to open planfile: %v", err)
		return 1
	}
	defer pf.Close()

	plan, err := pf.GetTfPlan()
	if err != nil {
		log.Printf("error: failed to get tfplan from planfile: %v", err)
		return 1
	}
	state, err := pf.GetTfState()
	if err != nil {
		log.Printf("error: failed to get tfstate from planfile: %v", err)
		return 1
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

	je := json.NewEncoder(os.Stdout)
	je.SetIndent("", "  ")
	if err := je.Encode(toRelayData(state)); err != nil {
		log.Printf("error: failed to write output: %v", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(run(os.Stdout, os.Stderr, os.Args))
}
