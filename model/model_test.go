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
