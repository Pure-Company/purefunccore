// IO composition example showing reader/writer transformations
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"

	pfc "github.com/Pure-Company/purefunccore"
)

func main() {
	// Example 1: Transform reader
	reader := pfc.ReadFunc(func(p []byte) (int, error) {
		return copy(p, []byte("hello world")), io.EOF
	}).
		Map(bytes.ToUpper).
		Map(func(b []byte) []byte {
			return []byte(">> " + string(b))
		})

	data, _ := io.ReadAll(reader)
	fmt.Println("Transformed:", string(data))

	// Example 2: Tee writer
	var buf1, buf2, buf3 bytes.Buffer

	writer := pfc.WriteFunc(buf1.Write).
		Tee(pfc.WriteFunc(buf2.Write)).
		Tee(pfc.WriteFunc(buf3.Write))

	writer.Write([]byte("broadcast message"))

	fmt.Println("Buffer 1:", buf1.String())
	fmt.Println("Buffer 2:", buf2.String())
	fmt.Println("Buffer 3:", buf3.String())

	log.Println("✅ IO composition works!")
}
