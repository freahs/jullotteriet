package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
)

type Args struct {
	Seed        int64
	MaxAttempts int
	Names       []string
	Constraints map[string][]string
}

func ParseArgs(args []string) (Args, error) {

	res := Args{
		Seed:        0,
		MaxAttempts: 100,
		Names:       make([]string, 0),
		Constraints: make(map[string][]string),
	}

	pop := func() string {
		arg := args[0]
		args = args[1:]
		return arg
	}

	for len(args) > 0 {
		arg := strings.ToLower(pop())
		if strings.HasPrefix(arg, "--") && len(args) == 0 {
			return res, fmt.Errorf("missing value for %v", arg)
		}

		switch arg {

		case "--file":
			path := pop()
			f, err := os.Open(path)
			if err != nil {
				return res, fmt.Errorf("error reading constraints file: %v", err)
			}
			err = json.NewDecoder(f).Decode(&res.Constraints)
			if err != nil {
				return res, fmt.Errorf("invalid constraings file: %v", err)
			}

		case "--seed":
			var err error
			res.Seed, err = strconv.ParseInt(pop(), 10, 64)
			if err != nil {
				return res, fmt.Errorf("invalid value for seed: %v", err)
			}

		case "--max-attempts":
			var err error
			res.MaxAttempts, err = strconv.Atoi(pop())
			if err != nil {
				return res, fmt.Errorf("invalid value for max-attempts: %v", err)
			}

		default:
			res.Names = append(res.Names, arg)
		}

	}

	for _, name := range res.Names {
		if _, ok := res.Constraints[name]; !ok {
			res.Constraints[name] = []string{}
		}
	}

	res.Names = make([]string, 0, len(res.Constraints))
	for name, _ := range res.Constraints {
		res.Names = append(res.Names, name)
	}

	sort.StringSlice(res.Names).Sort()

	return res, nil
}

func (a Args) Assign() (map[string]string, error) {

	for i := 0; i < a.MaxAttempts; i++ {
		if assigned, ok := a.assign(); ok {
			log.Printf("finished ordering (%v retries)", i)
			return assigned, nil
		}
		a.Seed++
	}
	return nil, fmt.Errorf("no suitable seed found")
}

func (a Args) assign() (map[string]string, bool) {
	names := make([]string, len(a.Names))
	copy(names, a.Names)

	rand.Seed(a.Seed)
	rand.Shuffle(len(names), func(i, j int) { names[i], names[j] = names[j], names[i] })

	assigned := make(map[string]string)

	for i, n1 := range names {
		n2 := names[(i+1)%len(names)]
		for _, n3 := range a.Constraints[n1] {
			if n2 == n3 {
				return nil, false
			}
		}
		assigned[n1] = n2
	}

	return assigned, true
}
