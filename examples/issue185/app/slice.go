package app

import (
	"fmt"
	"io"
)

// RunSlice (gosubc: slice)
//
//	flag: --input (default: nil) (description: "Input readers")
func RunSlice(inputs []io.Reader) error {
	for _, in := range inputs {
		b, err := io.ReadAll(in)
		if err != nil {
			return err
		}
		fmt.Printf("Read: %s\n", string(b))
	}
	return nil
}
