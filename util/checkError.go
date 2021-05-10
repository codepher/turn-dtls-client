package util

import (
	"fmt"
	"net"
)

// Check is a helper to throw errors in the examples
func Check(err error) {
	switch e := err.(type) {
	case nil:
	case (net.Error):
		if e.Temporary() {
			fmt.Printf("Warning: %v\n", err)
			return
		}

		fmt.Printf("net.Error: %v\n", err)
		panic(err)
	default:
		fmt.Printf("error: %v\n", err)
		panic(err)
	}
}
