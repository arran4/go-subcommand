package commentv1

import "testing"

func BenchmarkParseParamDetails(b *testing.B) {
	input := `(default: "foo", aliases: bar, baz, @1, -f, --flag)`
	for i := 0; i < b.N; i++ {
		parseParamDetails(input)
	}
}
