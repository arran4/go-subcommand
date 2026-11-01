package app

import (
	"fmt"
)

// Root is a subcommand `app` - Root optional value command.
//
// Flags:
//
//	label: (optional value) --label -l Optional string label.
//	verbose: (optional value) --verbose -v Optional boolean flag.
func Root(label string, verbose bool) {
	fmt.Printf("label=%q verbose=%t\n", label, verbose)
}

// Scalar is a subcommand `app scalar` - Scalar optional values with defaults.
//
// Flags:
//
//	str: (optional value) --str -s String scalar.
//	num: (optional value) --num -n Integer scalar.
//	def: (optional value; default: "fallback") --def Defaulted string scalar.
func Scalar(str string, num int, def string) {
	fmt.Printf("str=%q num=%d def=%q\n", str, num, def)
}

// Pointers is a subcommand `app pointers` - Pointer and double-pointer optional values.
//
// Flags:
//
//	ptr: (optional value) --ptr -p Optional string pointer.
//	num: (optional value) --num -n Optional int pointer.
//	tri: (optional value) --tri Optional double pointer.
func Pointers(ptr *string, num *int, tri **string) {
	ptrStr := "<nil>"
	if ptr != nil {
		ptrStr = fmt.Sprintf("%q", *ptr)
	}
	numStr := "<nil>"
	if num != nil {
		numStr = fmt.Sprintf("%d", *num)
	}
	triStr := "<nil>"
	if tri != nil {
		if *tri == nil {
			triStr = "<bare>"
		} else {
			triStr = fmt.Sprintf("%q", **tri)
		}
	}
	fmt.Printf("ptr=%s num=%s tri=%s\n", ptrStr, numStr, triStr)
}

// Slices is a subcommand `app slices` - Slice and pointer-slice optional values.
//
// Flags:
//
//	items: (optional value) --items -i Slice of strings.
//	ptrs: (optional value) --ptrs -p Lossless slice of string pointers.
func Slices(items []string, ptrs []*string) {
	ptrsFormatted := make([]string, 0, len(ptrs))
	for _, p := range ptrs {
		if p == nil {
			ptrsFormatted = append(ptrsFormatted, "<nil>")
		} else {
			ptrsFormatted = append(ptrsFormatted, fmt.Sprintf("%q", *p))
		}
	}
	fmt.Printf("items=%q ptrs=%v\n", items, ptrsFormatted)
}

// Scope is a subcommand `app scope` - Optional flag before positional arg.
//
// Flags:
//
//	opt: (optional value) --opt -o Optional string pointer.
//	pos: @1 Positional argument.
func Scope(opt *string, pos string) {
	optStr := "<nil>"
	if opt != nil {
		optStr = fmt.Sprintf("%q", *opt)
	}
	fmt.Printf("opt=%s pos=%q\n", optStr, pos)
}

// NullString is a consumer-owned nullable representation.
type NullString struct {
	String string
	Valid  bool
}

func ToNullString(values []*string) (*NullString, error) {
	if len(values) == 0 {
		return nil, nil
	}
	v := values[len(values)-1]
	if v == nil {
		return &NullString{Valid: false}, nil
	}
	return &NullString{String: *v, Valid: true}, nil
}

// Convert is a subcommand `app convert` - Generator conversion.
//
// Flags:
//
//	raw: (optional value) --raw -r Raw optional flags.
//	dest: (generator: ToNullString(raw)) Converted NullString.
func Convert(raw []*string, dest *NullString) {
	if dest == nil {
		fmt.Println("dest=<nil>")
	} else if !dest.Valid {
		fmt.Println("dest=<bare>")
	} else {
		fmt.Printf("dest=%q\n", dest.String)
	}
}

type Level int

func ParseLevel(s string) (Level, error) {
	switch s {
	case "debug":
		return 1, nil
	case "info":
		return 2, nil
	case "warn":
		return 3, nil
	case "":
		return 0, nil
	default:
		return 0, fmt.Errorf("unknown level %q", s)
	}
}

// Custom is a subcommand `app custom` - Custom parser optional flag.
//
// Flags:
//
//	level: (optional value; parser: ParseLevel) --level -l Log level.
func Custom(level *Level) {
	if level == nil {
		fmt.Println("level=<nil>")
	} else {
		fmt.Printf("level=%d\n", *level)
	}
}
