package go_subcommand

import (
	"fmt"
	"strings"
	"unicode"
)

// SanitizeToIdentifier converts a string into a valid Go identifier (CamelCase).
// It handles hyphens, underscores, and other non-alphanumeric characters by
// acting as delimiters for CamelCasing.
func SanitizeToIdentifier(name string) string {
	var builder strings.Builder
	nextUpper := true // First character should always be upper (Exported)

	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if nextUpper {
				builder.WriteRune(unicode.ToUpper(r))
				nextUpper = false
			} else {
				builder.WriteRune(r)
			}
		} else {
			// Treat any non-alphanumeric char as a delimiter
			nextUpper = true
		}
	}

	res := builder.String()
	// Ensure it doesn't start with a digit
	if len(res) > 0 && unicode.IsDigit(rune(res[0])) {
		res = "Cmd" + res
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
