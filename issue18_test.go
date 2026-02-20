package go_subcommand

import (
	"strings"
	"testing"
)

func TestIssue18_MoreBasicTypes(t *testing.T) {
	src := `package main

// MyCmd is a subcommand ` + "`app mycmd`" + `
func MyCmd(
	i64 int64,
	i32 int32,
	i16 int16,
	i8 int8,
	u uint,
	u64 uint64,
	u32 uint32,
	u16 uint16,
	u8 uint8,
	f64 float64,
	f32 float32,
	si64 []int64,
	pi64 *int64,
) {}
`
	fs := setupProject(t, src)
	writer := runGenerateInMemory(t, fs)

	cmdPath := "cmd/app/mycmd.go"
	content, ok := writer.Files[cmdPath]
	if !ok {
		t.Fatalf("Generated file not found: %s", cmdPath)
	}
	code := string(content)

	// Check for TODOs indicating missing implementation
	types := []string{
		"int64", "int32", "int16", "int8",
		"uint", "uint64", "uint32", "uint16", "uint8",
		"float64", "float32",
		"[]int64", "*int64",
	}

	for _, typ := range types {
		if strings.Contains(code, "TODO: Implement parsing for flag type "+typ) {
			t.Errorf("Missing parsing for flag type %s", typ)
		}
	}
}
