# orange

orange is a small Go library for interacting with range servers.

### Usage

Documentation is available via
[![GoDoc](https://godoc.org/github.com/karrick/orange?status.svg)](https://godoc.org/github.com/karrick/orange).

### Description

Querying range is a simple HTTP GET call, and Go already provides a
steller http library.  So why wrap it?  Well, either you write your
own wrapper or use one someone else has written, it's all the same to
me.  But I had to write the wrapper, so I figured I would at least
provide my implementation as a reference piece for others doing the
same.

In any event, this library

1. Guarantees HTTP connections can be re-used by always reading all
   body bytes if the Get succeeded.
1. Detects and parses the RangeException header, returning any error
   message encoded therein.
1. Converts response body to slice of strings.

There are four possible error types this library returns:

1. Raw error that the underlying Get method returned.
1. ErrStatusNotOK is returned when the response status code is not OK.
1. ErrRangeException is returned when the response headers includes
   'RangeException' header.
1. ErrParseException is returned by Client.Query when an error occurs
   while parsing the GET response.

### Example

#### Create a Client

Create a range client by specifying the desired configuration
parameters, then use the client.  See the `orange.Config` data
structure and field members to use a provided `http.Client` instance
or to customize the client's handling of retries.  The only required
parameter is the Servers field.

```Go
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
```

#### Sending Multiple Queries Simultaneously

Kind of a contrived example, but if you have multiple queries to send
and find the union of the results, you can do so in parallel by
calling the `Queries` method.

```Go
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

	var expressions []string

	// main loop
	fmt.Printf("> ")
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		expressions = append(expressions, scanner.Text())
		fmt.Printf("> ")
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	results, err := client.Queries(expressions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n", err)
		os.Exit(1)
	}
	fmt.Println(results)
}
```
