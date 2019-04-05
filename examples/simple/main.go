package main

import (
	"bufio"
	"fmt"
	"os"

	"github.com/karrick/orange"
)

func main() {
	// Create a range client.  Programs can list more than one server and
	// include other options.  See Config structure documentation for specifics.
	client, err := orange.NewClient(&orange.Config{
		Servers: []string{"localhost:8081"},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}

	// Example program main loop reads query from standard input, queries the
	// range server, then prints the response.
	fmt.Printf("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		values, err := client.Query(scanner.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			fmt.Printf("> ")
			continue
		}
		fmt.Printf("%v\n> ", values)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}
