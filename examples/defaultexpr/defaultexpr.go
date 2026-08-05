package defaultexpr

import (
	"fmt"
	"math"
	"runtime"
)

//go:generate sh -c "command -v gosubc >/dev/null 2>&1 && gosubc generate --clean || go run github.com/arran4/go-subcommand/cmd/gosubc generate --clean"

var (
	_ = math.MaxInt32
	_ = runtime.NumCPU
)


// Root is a subcommand `app`
//
// Flags:
//
//	cores: (default: runtime.NumCPU()) Number of cores
//	limit: (default: math.MaxInt32) Max limit
func Root(cores int, limit int) {
	fmt.Printf("cores: %d, limit: %d\n", cores, limit)
}


