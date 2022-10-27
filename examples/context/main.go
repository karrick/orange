package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/karrick/orange"
)

func main() {
	optTimeout := flag.Duration("timeout", 0, "timeout duration for the query")
	flag.Parse()

	// Create a range client.  Programs can list more than one server and
	// include other options.  See Config structure documentation for specifics.
	client, err := orange.NewClient(&orange.Config{
		Servers: []string{"localhost:8081"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	if *optTimeout > 0 {
		var done func()
		ctx, done = context.WithTimeout(ctx, *optTimeout)
		defer done()
	}

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "USAGE: %s [-timeout DURATION] q1 q2\n")
		os.Exit(1)
	}

	values, err := client.QueryCtx(ctx, strings.Join(flag.Args(), ","))
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}

	fmt.Println(values)
}
