package parsers

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/arran4/strings2"
)

// ToKebabCase converts a CamelCase string to kebab-case.
// It handles acronyms (e.g. JSONData -> json-data) and simple cases (CamelCase -> camel-case).
func ToKebabCase(s string) string {
	res, err := strings2.ToKebab(s, strings2.WithNumberSplitting(true))
	if err != nil {
		return strings.ToLower(s)
	}
	return strings.ToLower(res)
}

// SanitizeToIdentifier converts a string into a valid Go identifier (CamelCase).
// It handles hyphens, underscores, and other non-alphanumeric characters by
// acting as delimiters for CamelCasing.
func SanitizeToIdentifier(name string) string {
	var builder strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	input := builder.String()

	res, err := strings2.ToPascal(input)
	if err != nil {
		res = name
	}

	builder.Reset()

	for _, r := range res {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}

	res = builder.String()
	// Ensure it doesn't start with a digit
	if len(res) > 0 {
		r, _ := utf8.DecodeRuneInString(res)
		if unicode.IsDigit(r) {
			res = "Cmd" + res
		}
	}
	// Fallback for empty result
	if len(res) == 0 {
		return "Cmd"
	}

	return res
}

// NameAllocator manages the assignment of unique identifier names.
type NameAllocator struct {
	used map[string]bool
}

// NewNameAllocator creates a new allocator with pre-reserved names.
func NewNameAllocator() *NameAllocator {
	na := &NameAllocator{
		used: make(map[string]bool),
	}
	// Reserve names used in generated code
	reserved := []string{
		"Cmd", "RootCmd", "UserError",
		"NewRoot", "NewUserError", "executeUsage",
		"main", "init", // Standard Go
	}
	for _, r := range reserved {
		na.used[r] = true
	}
	return na
}

// Allocate generates a unique name based on the input string.
// It sanitizes the input and handles collisions by appending numbers.
func (na *NameAllocator) Allocate(input string) string {
	base := SanitizeToIdentifier(input)
	name := base
	count := 2
	for na.used[name] {
		name = fmt.Sprintf("%s%d", base, count)
		count++
	}
	na.used[name] = true
	return name
}
