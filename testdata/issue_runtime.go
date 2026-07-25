package app

// App is a subcommand `app`.
//
// Flags:
//
//	config: (required) --config Configuration path
func App(config string) {}

// Parent is a subcommand `app parent`.
//
// Flags:
//
//	dir: --dir Parent directory
func Parent(dir string) {}

// Child is a subcommand `app parent child`.
//
// Flags:
//
//	dir: --dir (from parent)
//	z: -z Enable z
//	x: -x Enable x
//	w: -w Enable w
//	q: -q Value q
//	value: -v Value
//	values: -V Repeatable values
//	ptr: --ptr Nullable integer
//	parsed: (parser: "example.com/e2e/parserpkg".Parse) --parsed Imported parser
//	localParsed: (parser: ParseLocal) --local-parsed Local parser
//	generated: (generator: "example.com/e2e/parserpkg".Gen) Generated dependency
func Child(d string, z, x, w bool, q string, value string, values []string, ptr *int, parsed, localParsed, generated string) {
}

func ParseLocal(value string) (string, error) {
	return "local:" + value, nil
}
