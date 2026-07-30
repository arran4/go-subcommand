package go_subcommand

import (
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
		t.Errorf("Expected 'CUSTOM USAGE ALIAS', got %q", string(data))
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
		t.Errorf("Expected 'FOLDER USAGE OVERLAY', got %q", string(data))
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

	expected := "CUSTOM USAGE TEMPLATE OVERLAY: {{.FullUsageString}}\n"
	if string(data) != expected {
		t.Errorf("Expected %q, got %q", expected, string(data))
	}

	manData, err := fs.ReadFile(overlay, "templates/cmd/templates/man.gotmpl")
	if err != nil {
		t.Fatalf("ReadFile man template from overlay failed: %v", err)
	}

	expectedMan := "CUSTOM MAN TEMPLATE OVERLAY\n"
	if string(manData) != expectedMan {
		t.Errorf("Expected %q, got %q", expectedMan, string(manData))
	}
}
