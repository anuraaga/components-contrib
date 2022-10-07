package main

import "github.com/wapc/wapc-guest-tinygo"

func main() {
	wapc.RegisterFunctions(wapc.Functions{"handle": handle})
}

// rewrite returns a new URI if necessary.
func handle(_ []byte) ([]byte, error) {
	p, err := wapc.HostCall("", "http-handler", "get-path", []byte{})
	if err != nil {
		return nil, err
	}
	if string(p) == "/v1.0/hi" {
		_, err = wapc.HostCall("", "http-handler", "set-path", []byte("/v1.0/hello"))
		if err != nil {
			return nil, err
		}
	}
	_, err = wapc.HostCall("", "http-handler", "next", []byte{})
	return nil, err
}
