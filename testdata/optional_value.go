package app

import (
	"errors"
	"fmt"
)

// OptionalLabel is a consumer-owned value type. It deliberately has no
// dependency on gosubc.
type OptionalLabel struct {
	Present  bool
	HasValue bool
	Value    string
}

func (o *OptionalLabel) SetOptionalValue(present, hasValue bool, value string) error {
	o.Present, o.HasValue, o.Value = present, hasValue, value
	return nil
}

// Root is a subcommand `app`.
// Flags:
//
//	label: (optional value) --label -l A consumer-owned three-state label.
func Root(label OptionalLabel) {
	fmt.Printf("present=%t value=%t label=%q\n", label.Present, label.HasValue, label.Value)
}

// OptionalPointers proves that consumers may use pointers and repeated values
// in their own durable optional-value representation.
type OptionalPointers struct {
	Bare   int
	Values []*string
}

func (o *OptionalPointers) SetOptionalValue(_ bool, hasValue bool, value string) error {
	if !hasValue {
		o.Bare++
		return nil
	}
	val := value
	o.Values = append(o.Values, &val)
	return nil
}

// Pointers is a subcommand `app pointers`.
// Flags:
//
//	items: (optional value) --items -i Repeated optional pointer values.
func Pointers(items OptionalPointers) {
	values := make([]string, 0, len(items.Values))
	for _, value := range items.Values {
		values = append(values, *value)
	}
	fmt.Printf("bare=%d values=%q\n", items.Bare, values)
}

// OptionalValidation validates values and returns an error on invalid input.
type OptionalValidation struct {
	Present  bool
	HasValue bool
	Value    string
}

func (o *OptionalValidation) SetOptionalValue(present, hasValue bool, value string) error {
	if hasValue && value == "invalid" {
		return errors.New("invalid parameter payload")
	}
	o.Present, o.HasValue, o.Value = present, hasValue, value
	return nil
}

// Validate is a subcommand `app validate`.
// Flags:
//
//	strict: (optional value) --strict -s Validated optional flag.
func Validate(strict OptionalValidation) {
	fmt.Printf("strict present=%t value=%t value=%q\n", strict.Present, strict.HasValue, strict.Value)
}

// Ptr is a subcommand `app ptr`.
// Flags:
//
//	opt: (optional value) --opt -o Optional pointer flag.
func Ptr(opt *OptionalLabel) {
	if opt == nil {
		fmt.Println("opt is nil")
	} else {
		fmt.Printf("opt present=%t value=%t value=%q\n", opt.Present, opt.HasValue, opt.Value)
	}
}

// Scope is a subcommand `app scope`.
// Flags:
//
//	label: (optional value) --label -l Optional flag before positional arg.
//	pos: @1 Positional argument.
func Scope(label OptionalLabel, pos string) {
	fmt.Printf("scope present=%t value=%t label=%q pos=%q\n", label.Present, label.HasValue, label.Value, pos)
}
