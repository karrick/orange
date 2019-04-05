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
		response, err := client.Query(scanner.Text())
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
			fmt.Printf("> ")
			continue
		}

		// The Query method returns a Response instance that can either return
		// the raw byte slice from reading the range response, or a slice of
		// strings, each string representing one of the results.  Using Split to
		// return a slice of streings is the more common use case, but the Bytes
		// method is provided for programs that want need the raw byte slice,
		// such as a cache.
		fmt.Printf("%v\n> ", response.Split())
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
	}
}
