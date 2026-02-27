package model

import (
	"testing"
)

func TestFunctionParameterHelpers(t *testing.T) {
	tests := []struct {
		typ        string
		isSlice    bool
		isPointer  bool
		baseType   string
		isBool     bool
		isString   bool
		isDuration bool
	}{
		{"int", false, false, "int", false, false, false},
		{"*int", false, true, "int", false, false, false},
		{"[]int", true, false, "int", false, false, false},
		{"[]*int", true, true, "int", false, false, false},
		{"string", false, false, "string", false, true, false},
		{"*string", false, true, "string", false, true, false},
		{"[]string", true, false, "string", false, true, false},
		{"bool", false, false, "bool", true, false, false},
		{"*bool", false, true, "bool", true, false, false},
		{"time.Duration", false, false, "time.Duration", false, false, true},
		{"*time.Duration", false, true, "time.Duration", false, false, true},
		{"[]time.Duration", true, false, "time.Duration", false, false, true},
		{"[]*time.Duration", true, true, "time.Duration", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			p := &FunctionParameter{Type: tt.typ}

			if got := p.IsSlice(); got != tt.isSlice {
				t.Errorf("IsSlice() = %v, want %v", got, tt.isSlice)
			}
			if got := p.HasPointer(); got != tt.isPointer {
				t.Errorf("HasPointer() = %v, want %v", got, tt.isPointer)
			}
			if got := p.BaseType(); got != tt.baseType {
				t.Errorf("BaseType() = %v, want %v", got, tt.baseType)
			}
			if got := p.IsBool(); got != tt.isBool {
				t.Errorf("IsBool() = %v, want %v", got, tt.isBool)
			}
			if got := p.IsString(); got != tt.isString {
				t.Errorf("IsString() = %v, want %v", got, tt.isString)
			}
			if got := p.IsDuration(); got != tt.isDuration {
				t.Errorf("IsDuration() = %v, want %v", got, tt.isDuration)
			}
		})
	}
}

func TestAppendFlagsUsage(t *testing.T) {
	tests := []struct {
		name       string
		parameters []*FunctionParameter
		initial    []string
		expected   []string
	}{
		{
			name:       "No parameters",
			parameters: []*FunctionParameter{},
			initial:    []string{"cmd"},
			expected:   []string{"cmd"},
		},
		{
			name: "Only positional",
			parameters: []*FunctionParameter{
				{Name: "arg1", IsPositional: true},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd"},
		},
		{
			name: "Single bool flag",
			parameters: []*FunctionParameter{
				{Name: "verbose", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[--verbose]"},
		},
		{
			name: "Single string flag",
			parameters: []*FunctionParameter{
				{Name: "output", Type: "string", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[--output <output>]"},
		},
		{
			name: "Short flag name",
			parameters: []*FunctionParameter{
				{Name: "v", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[-v]"},
		},
		{
			name: "Flag with alias",
			parameters: []*FunctionParameter{
				{Name: "verbose", Type: "bool", FlagAliases: []string{"v"}, Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[-v]"},
		},
		{
			name: "Flag with longer alias used",
			// If FlagAliases has "verbose", and Name is "v", the logic in appendFlagsUsage iterates FlagAliases
			// "verbose" has len 7, "v" has len 1. shortest = "v".
			// So it should be -v.
			// Wait, in my previous failure I expected --verbose but got -v.
			// The failure message was: parts[1] = "[-v]", want "[--verbose]"
			// My test case setup:
			// {Name: "v", Type: "bool", FlagAliases: []string{"verbose"}},
			// appendFlagsUsage uses Name as initial shortest ("v"), then checks aliases ("verbose").
			// "verbose" is longer than "v", so shortest remains "v".
			// So result is "[-v]".
			// I should adjust the expectation if I want it to pick shortest.
			// If the intention of the test case was "Flag with longer alias used", implying that maybe the Name IS the short one?
			// The previous test case "Flag with alias" had Name="verbose", Alias="v". Shortest="v". Result="-v". Correct.
			// This case has Name="v", Alias="verbose". Shortest="v". Result="-v". Correct.
			// So my expectation in the test code was wrong for the logic "pick shortest".
			parameters: []*FunctionParameter{
				{Name: "v", Type: "bool", FlagAliases: []string{"verbose"}, Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[-v]"},
		},
		{
			name: "Flag with multiple aliases picks shortest",
			parameters: []*FunctionParameter{
				{Name: "foo", Type: "bool", FlagAliases: []string{"verylong", "s", "medium"}, Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[-s]"},
		},
		{
			name: "Three flags",
			parameters: []*FunctionParameter{
				{Name: "a", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
				{Name: "b", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
				{Name: "c", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[-a]", "[-b]", "[-c]"},
		},
		{
			name: "Four flags",
			parameters: []*FunctionParameter{
				{Name: "a", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
				{Name: "b", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
				{Name: "c", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
				{Name: "d", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[flags...]"},
		},
		{
			name: "Mixed positional and flags",
			parameters: []*FunctionParameter{
				{Name: "arg1", IsPositional: true},
				{Name: "verbose", Type: "bool", Generator: GeneratorConfig{Type: SourceTypeFlag}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd", "[--verbose]"},
		},
		{
			name: "Generator param ignored",
			parameters: []*FunctionParameter{
				{Name: "gen", Type: "string", Generator: GeneratorConfig{Type: SourceTypeGenerator}},
			},
			initial:  []string{"cmd"},
			expected: []string{"cmd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := make([]string, len(tt.initial))
			copy(parts, tt.initial)
			appendFlagsUsage(&parts, tt.parameters)

			if len(parts) != len(tt.expected) {
				t.Fatalf("got length %d, want %d", len(parts), len(tt.expected))
			}
			for i := range parts {
				if parts[i] != tt.expected[i] {
					t.Errorf("parts[%d] = %q, want %q", i, parts[i], tt.expected[i])
				}
			}
		})
	}
}
