package go_subcommand

import (
	"bytes"
	_ "embed"
	"io/fs"
	"testing"
	"testing/fstest"
)

//go:embed testdata/overlay_templates.txtar
var overlayTxtarData []byte

func TestBuildOverlayFS_Alias(t *testing.T) {
	inMemFS := fstest.MapFS{
		"my_usage.gotmpl": &fstest.MapFile{Data: []byte("CUSTOM USAGE ALIAS")},
	}

	overlay, err := buildOverlayFS(TemplatesFS, []string{"usage=my_usage.gotmpl"}, inMemFS)
	if err != nil {
		t.Fatalf("buildOverlayFS failed: %v", err)
	}

	data, err := fs.ReadFile(overlay, "templates/cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("ReadFile from overlay failed: %v", err)
	}
	if string(data) != "CUSTOM USAGE ALIAS" {
		t.Errorf("expected customized usage, got %s", string(data))
	}
}

func TestBuildOverlayFS_Folder(t *testing.T) {
	inMemFS := fstest.MapFS{
		"custom_dir/cmd/templates/usage.txt.gotmpl": &fstest.MapFile{Data: []byte("FOLDER USAGE OVERLAY")},
	}

	overlay, err := buildOverlayFS(TemplatesFS, []string{"custom_dir"}, inMemFS)
	if err != nil {
		t.Fatalf("buildOverlayFS failed: %v", err)
	}

	data, err := fs.ReadFile(overlay, "templates/cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("ReadFile from overlay failed: %v", err)
	}
	if string(data) != "FOLDER USAGE OVERLAY" {
		t.Errorf("expected customized usage, got %q", string(data))
	}
}

func TestBuildOverlayFS_Txtar(t *testing.T) {
	inMemFS := fstest.MapFS{
		"templates.txtar": &fstest.MapFile{Data: overlayTxtarData},
	}

	overlay, err := buildOverlayFS(TemplatesFS, []string{"templates.txtar"}, inMemFS)
	if err != nil {
		t.Fatalf("buildOverlayFS failed: %v", err)
	}

	data, err := fs.ReadFile(overlay, "templates/cmd/templates/usage.txt.gotmpl")
	if err != nil {
		t.Fatalf("ReadFile from overlay failed: %v", err)
	}
	if want := "CUSTOM USAGE TEMPLATE OVERLAY: {{.FullUsageString}}\n"; string(data) != want {
		t.Errorf("expected %q, got %q", want, string(data))
	}

	data, err = fs.ReadFile(overlay, "templates/cmd/templates/man.gotmpl")
	if err != nil {
		t.Fatalf("ReadFile man template from overlay failed: %v", err)
	}
	if want := "CUSTOM MAN TEMPLATE OVERLAY\n"; string(data) != want {
		t.Errorf("expected %q, got %q", want, string(data))
	}
}

func TestBuildOverlayFS_Composition(t *testing.T) {
	baseFS := fstest.MapFS{
		"templates/common.gotmpl": &fstest.MapFile{
			Data: []byte(`{{define "common_a"}}base_a{{end}} {{define "common_b"}}base_b{{end}}`),
		},
	}

	readFS := fstest.MapFS{
		"replacement1.txtar": &fstest.MapFile{
			Data: []byte("-- common.gotmpl --\n{{define \"common_a\"}}first_a{{end}} {{define \"common_c\"}}first_c{{end}}\n"),
		},
		"replacement2.txtar": &fstest.MapFile{
			Data: []byte("-- common.gotmpl --\n{{define \"common_a\"}}second_a{{end}} {{define \"common_d\"}}second_d{{end}}\n"),
		},
	}

	overlay, err := buildOverlayFS(baseFS, []string{"replacement1.txtar", "replacement2.txtar"}, readFS)
	if err != nil {
		t.Fatalf("buildOverlayFS failed: %v", err)
	}

	tmpl, err := ParseTemplates(overlay)
	if err != nil {
		t.Fatalf("ParseTemplates failed: %v", err)
	}

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{"override_latest_wins", "common_a", "second_a"},
		{"base_preserved", "common_b", "base_b"},
		{"middle_preserved", "common_c", "first_c"},
		{"top_preserved", "common_d", "second_d"},
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
