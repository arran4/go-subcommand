package go_subcommand

import (
	"bytes"
	"testing"
	"testing/fstest"
)

func TestParseTemplates_Composition(t *testing.T) {
	baseFS := fstest.MapFS{
		"templates/common.gotmpl": &fstest.MapFile{
			Data: []byte(`{{define "common_a"}}base_a{{end}} {{define "common_b"}}base_b{{end}}`),
		},
	}

	overlay1 := fstest.MapFS{
		"templates/common.gotmpl": &fstest.MapFile{
			Data: []byte(`{{define "common_a"}}overlay_a1{{end}} {{define "common_c"}}overlay_c{{end}}`),
		},
	}

	overlay2 := fstest.MapFS{
		"templates/common.gotmpl": &fstest.MapFile{
			Data: []byte(`{{define "common_a"}}overlay_a2{{end}} {{define "common_d"}}overlay_d{{end}}`),
		},
	}

	fsys := &overlayFS{
		base: &overlayFS{
			base:    baseFS,
			overlay: overlay1,
		},
		overlay: overlay2,
	}

	tmpl, err := ParseTemplates(fsys)
	if err != nil {
		t.Fatalf("ParseTemplates failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"override_latest_wins", "common_a", "overlay_a2"},
		{"base_preserved", "common_b", "base_b"},
		{"middle_preserved", "common_c", "overlay_c"},
		{"top_preserved", "common_d", "overlay_d"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tmpl.ExecuteTemplate(&buf, tt.template, nil); err != nil {
				t.Fatalf("failed to execute template %q: %v", tt.template, err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
