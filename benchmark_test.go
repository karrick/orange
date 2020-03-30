package orange

import (
	"context"
	"net/http"
	"testing"
)

const lineCount = 129

var largeResponse []byte

func init() {
	largeResponse = make([]byte, 1<<15) // 32 KiB
	for i := 0; i < len(largeResponse); i++ {
		largeResponse[i] = byte(i)
	}
}

func BenchmarkLineSplitting(b *testing.B) {
	h := func(w http.ResponseWriter, r *http.Request) {
		w.Write(largeResponse)
	}

	b.Run("bufio scanner", func(b *testing.B) {
		withTestClient(b, h, func(client *Client) {
			for i := 0; i < b.N; i++ {
				values, err := client.QueryCtx(context.Background(), "foo")
				if err != nil {
					b.Fatal(err)
				}
				if got, want := len(values), lineCount; got != want {
					b.Fatalf("GOT: %v; WANT: %v", got, want)
				}
			}
		})
	})
}
