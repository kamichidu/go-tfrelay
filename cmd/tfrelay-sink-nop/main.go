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
	colog.SetPrefix("[ tfrelay-sink-nop ] ")

	var (
		pretty bool
	)
	flag.BoolVar(&pretty, "pretty", pretty, "")
	flag.Parse()

	jd := json.NewDecoder(os.Stdin)
	je := json.NewEncoder(os.Stdout)
	if pretty {
		je.SetIndent("", "  ")
	}
	var data json.RawMessage
	if err := jd.Decode(&data); err != nil {
		log.Panic(err)
	}
	if err := je.Encode(data); err != nil {
		log.Panic(err)
	}
}
