package commentv1

import (
	"reflect"
	"testing"
)

func TestExtractParamAttributes(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		wantAttrs string
		wantClean string
	}{
		{
			name:      "Start Block",
			text:      "(required) Description",
			wantAttrs: "required",
			wantClean: "Description",
		},
		{
			name:      "End Block",
			text:      "Description (required)",
			wantAttrs: "required",
			wantClean: "Description",
		},
		{
			name:      "Middle Block Ignored",
			text:      "Description (required) text",
			wantAttrs: "",
			wantClean: "Description (required) text",
		},
		{
			name:      "Nested Parens Start",
			text:      "(parser: func(a,b)) Description",
			wantAttrs: "parser: func(a,b)",
			wantClean: "Description",
		},
		{
			name:      "Nested Parens End",
			text:      "Description (parser: func(a,b))",
			wantAttrs: "parser: func(a,b)",
			wantClean: "Description",
		},
		{
			name:      "Start Priority",
			text:      "(A) Description (B)",
			wantAttrs: "A",
			wantClean: "Description (B)",
		},
		{
			name:      "Complex Attributes",
			text:      "(parser: NewThing;required;global;aka nt) Description",
			wantAttrs: "parser: NewThing;required;global;aka nt",
			wantClean: "Description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAttrs, gotClean := extractParamAttributes(tt.text)
			if gotAttrs != tt.wantAttrs {
				t.Errorf("extractParamAttributes() attrs = %v, want %v", gotAttrs, tt.wantAttrs)
			}
			if gotClean != tt.wantClean {
				t.Errorf("extractParamAttributes() clean = %v, want %v", gotClean, tt.wantClean)
			}
		})
	}
}

func TestParseAttributes(t *testing.T) {
	tests := []struct {
		name      string
		attrs     string
		wantParam ParsedParam
	}{
		{
			name:  "Required",
			attrs: "required",
			wantParam: ParsedParam{
				Required: true,
			},
		},
		{
			name:  "Global",
			attrs: "global",
			wantParam: ParsedParam{
				Inherited: true,
			},
		},
		{
			name:  "Parser Simple",
			attrs: "parser: MyFunc",
			wantParam: ParsedParam{
				ParserFunc: "MyFunc",
			},
		},
		{
			name:  "Parser Package",
			attrs: "parser: pkg.MyFunc",
			wantParam: ParsedParam{
				ParserPkg:  "pkg",
				ParserFunc: "MyFunc",
			},
		},
		{
			name:  "Parser String Import",
			attrs: `parser: "github.com/foo/bar".MyFunc`,
			wantParam: ParsedParam{
				ParserPkg:  "github.com/foo/bar",
				ParserFunc: "MyFunc",
			},
		},
		{
			name:  "Generator",
			attrs: "generator: MyGen",
			wantParam: ParsedParam{
				Generator: "MyGen",
			},
		},
		{
			name:  "Alias",
			attrs: "aka: f, foo",
			wantParam: ParsedParam{
				Flags: []string{"f", "foo"},
			},
		},
		{
			name:  "Multiple",
			attrs: "required; global; parser: func; aka: f",
			wantParam: ParsedParam{
				Required:   true,
				Inherited:  true,
				ParserFunc: "func",
				Flags:      []string{"f"},
			},
		},
		{
			name:  "Inherited",
			attrs: "inherited",
			wantParam: ParsedParam{
				Inherited: true,
			},
		},
		{
			name:  "Default",
			attrs: "default: 123",
			wantParam: ParsedParam{
				Default: "123",
			},
		},
		{
			name:  "Default Quoted",
			attrs: `default: "foo"`,
			wantParam: ParsedParam{
				Default: "foo",
			},
		},
		{
			name:  "Mixed Legacy Comma",
			attrs: `-f, default: false`,
			wantParam: ParsedParam{
				Flags:   []string{"f"},
				Default: "false",
			},
		},
		{
			name:  "Mixed Parser with Comma",
			attrs: `parser: func(a,b); required`,
			wantParam: ParsedParam{
				ParserFunc: "func(a,b)",
				Required:   true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p ParsedParam
			parseAttributes(tt.attrs, &p)
			if !reflect.DeepEqual(p, tt.wantParam) {
				t.Errorf("parseAttributes() = %+v, want %+v", p, tt.wantParam)
			}
		})
	}
}

func TestParseParamDetails_Integration(t *testing.T) {
	tests := []struct {
		name string
		text string
		want ParsedParam
	}{
		{
			name: "Start Block",
			text: "(required) Description",
			want: ParsedParam{
				Required:    true,
				Description: "Description",
			},
		},
		{
			name: "End Block",
			text: "Description (global)",
			want: ParsedParam{
				Inherited:   true,
				Description: "Description",
			},
		},
		{
			name: "Mixed with Flag",
			text: "(required) -f Description",
			want: ParsedParam{
				Required:    true,
				Flags:       []string{"f"},
				Description: "Description",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseParamDetails(tt.text)
			if got.Required != tt.want.Required {
				t.Errorf("Required = %v, want %v", got.Required, tt.want.Required)
			}
			if got.Inherited != tt.want.Inherited {
				t.Errorf("Inherited = %v, want %v", got.Inherited, tt.want.Inherited)
			}
			if len(got.Flags) != len(tt.want.Flags) {
				t.Errorf("Flags len = %d, want %d", len(got.Flags), len(tt.want.Flags))
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
		})
	}
}
