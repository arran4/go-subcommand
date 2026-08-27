package go_subcommand

import (
	"reflect"
	"testing"
)

func TestDeduplicateAndSortImports(t *testing.T) {
	imports := []templateImport{
		{Path: "fmt"},
		{Path: "os"},
		{Path: "errors"},
		{Path: "fmt"}, // Duplicate
		{Path: "example.com/project/pkg"},
		{Path: "time"},
		{Path: "github.com/someone/otherpkg"},
	}

	got := deduplicateAndSortImports(imports)

	wantStandard := []templateImport{
		{Path: "errors"},
		{Path: "fmt"},
		{Path: "os"},
		{Path: "time"},
	}

	wantOther := []templateImport{
		{Path: "example.com/project/pkg"},
		{Path: "github.com/someone/otherpkg"},
	}

	if !reflect.DeepEqual(got.Standard, wantStandard) {
		t.Errorf("Standard imports got: %v, want: %v", got.Standard, wantStandard)
	}

	if !reflect.DeepEqual(got.Other, wantOther) {
		t.Errorf("Other imports got: %v, want: %v", got.Other, wantOther)
	}
}
