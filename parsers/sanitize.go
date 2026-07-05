package parsers

import (
	"fmt"
	"github.com/arran4/strings2"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ToKebabCase converts a CamelCase string to kebab-case.
// It handles acronyms (e.g. JSONData -> json-data) and simple cases (CamelCase -> camel-case).
func ToKebabCase(s string) string {
	words, err := strings2.Parse(s, strings2.WithNumberMode(strings2.NumberModeMergeWithWord))
	if err != nil {
		return s
	}
	res, _ := strings2.ToKebabCase(words, strings2.OptionCaseMode(strings2.CMWhispering))
	return res
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
			builder.WriteRune(' ')
		}
	}

	words, err := strings2.Parse(builder.String())
	if err != nil || len(words) == 0 {
		return "Cmd"
	}
	res, _ := strings2.ToPascalCase(words)

	// Fallback for empty result
	if len(res) == 0 {
		return "Cmd"
	}
	// Ensure it doesn't start with a digit
	r, _ := utf8.DecodeRuneInString(res)
	if unicode.IsDigit(r) {
		res = "Cmd" + res
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
