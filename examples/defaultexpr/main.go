package main

import (
	"fmt"
	"math"
	"runtime"
)

// Root is a subcommand `app`
// Flags:
//   cores: (default: runtime.NumCPU()) Number of cores
//   limit: (default: math.MaxInt32) Max limit
func Root(cores int, limit int) {
	fmt.Printf("cores: %d, limit: %d\n", cores, limit)
}
