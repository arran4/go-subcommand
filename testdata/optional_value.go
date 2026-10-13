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
	o.Values = append(o.Values, &value)
	return nil
}

// Pointers is a subcommand `app pointers`.
// CLI-Parser: plus-minus
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
