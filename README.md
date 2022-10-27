# orange

orange is a Go library for interacting with range servers.

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
   body bytes if the HTTP request succeeded.
1. Detects and parses the RangeException header, returning any error
   message encoded therein.
1. Returns response as either parsed slice of strings or an unparsed
   slice of bytes or the HTTP response payload.
1. Allows using either a client provided http.Client, or uses a
   http.Client with default timeout settings.
1. Optionally retries queries that fail when RetryCount is greater
   than 0 and an optional RetryCallback function parameter.

There are three possible error types this library returns:

1. Raw error that the HTTP GET method returned.
1. ErrStatusNotOK is returned when the response status code is not OK.
1. ErrRangeException is returned when the response headers includes
   'RangeException' header.

### Examples

Create a range client by specifying the desired configuration
parameters, then use the client.  See the `orange.Config` data
structure and field members to use a provided `http.Client` instance
or to customize the client's handling of retries.  The only required
parameter is the Servers field.

#### Query / QueryCtx

Query and QueryCtx returns a slice of strings corresponding to the range server
response.

```Go
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
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		values, err := client.QueryCtx(ctx, scanner.Text())
		defer cancel()

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
```
