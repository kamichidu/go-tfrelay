package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"comail.io/go/colog"
	"github.com/google/uuid"
	"github.com/kamichidu/go-tfrelay"
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

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-source-planfile ] ")

	flag.Parse()

	pf, err := openPlanfile(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	defer pf.Close()

	plan, err := pf.GetTfPlan()
	if err != nil {
		panic(err)
	}
	state, err := pf.GetTfState()
	if err != nil {
		panic(err)
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
		panic(err)
	}
}
