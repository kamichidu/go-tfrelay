package main

import (
	"encoding/json"
	"flag"
	"os"

	"comail.io/go/colog"
	"github.com/kamichidu/go-tfrelay"
)

func main() {
	colog.Register()
	colog.SetPrefix("[ tfrelay-transform-groupby ] ")

	flag.Parse()

	var data tfrelay.RelayData
	jd := json.NewDecoder(os.Stdin)
	if err := jd.Decode(&data); err != nil {
		panic(err)
	}

	// group by resource.type
	memo := map[string]tfrelay.Resource{}
	for _, v := range data.Resources {
		dup, ok := memo[v.Type]
		if ok {
			v.Name = v.Name + "/" + dup.Name
			v.Instances = append(v.Instances, dup.Instances...)
		}
		memo[v.Type] = v
	}
	data.Resources = make([]tfrelay.Resource, 0, len(memo))
	for _, v := range memo {
		data.Resources = append(data.Resources, v)
	}

	je := json.NewEncoder(os.Stdout)
	if err := je.Encode(&data); err != nil {
		panic(err)
	}
}
