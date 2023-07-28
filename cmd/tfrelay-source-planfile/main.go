package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"comail.io/go/colog"
)

func main() {
	colog.Register()

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
	if err := je.Encode(state); err != nil {
		panic(err)
	}
}
