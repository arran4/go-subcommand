package app

import "fmt"

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
