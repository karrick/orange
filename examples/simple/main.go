package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/karrick/orange"
)

func main() {
	// create a range querier; could list additional servers or include other
	// options as well
	client, err := orange.NewClient(&orange.Config{
		Servers: []string{"localhost:8081"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	// main loop
	fmt.Printf("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		expression := scanner.Text()
		results, err := client.Query(expression)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			fmt.Printf("> ")
			continue
		}
		fmt.Printf("%s\n> ", results)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}
