package go_subcommand

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

// buildOverlayFS creates an fs.FS from replaceTemplates flags.
func buildOverlayFS(baseFS fs.FS, replaceTemplates []string) (fs.FS, error) {
	if len(replaceTemplates) == 0 {
		return baseFS, nil
	}

	overlay := fstest.MapFS{}

	for _, replace := range replaceTemplates {
		if strings.Contains(replace, "=") {
			// format: alias=file
			parts := strings.SplitN(replace, "=", 2)
			alias := parts[0]
			file := parts[1]

			content, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("failed to read template replacement file %s: %w", file, err)
			}

			// Map alias to templates path
			targetPath := ""
			switch alias {
			case "usage":
				targetPath = "templates/cmd/templates/usage.txt.gotmpl"
			case "man":
				targetPath = "templates/cmd/templates/man.gotmpl"
			case "cmd":
				targetPath = "templates/cmd/cmd.go.gotmpl"
			case "root":
				targetPath = "templates/cmd/root.go.gotmpl"
			case "templates":
				targetPath = "templates/cmd/templates/templates.go.gotmpl"
			default:
				targetPath = "templates/" + alias
			}

			overlay[targetPath] = &fstest.MapFile{Data: content}
		} else {
			// format: folder or txtar
			stat, err := os.Stat(replace)
			if err != nil {
				return nil, fmt.Errorf("failed to stat template replacement %s: %w", replace, err)
			}

			if stat.IsDir() {
				// Overlay folder
				err := filepath.WalkDir(replace, func(path string, d fs.DirEntry, err error) error {
					if err != nil || d.IsDir() {
						return err
					}
					rel, _ := filepath.Rel(replace, path)
					content, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					// Map to templates/
					overlay[filepath.ToSlash(filepath.Join("templates", rel))] = &fstest.MapFile{Data: content}
					return nil
				})
				if err != nil {
					return nil, fmt.Errorf("failed to walk template replacement folder %s: %w", replace, err)
				}
			} else {
				// Read as txtar
				content, err := os.ReadFile(replace)
				if err != nil {
					return nil, err
				}
				archive := txtar.Parse(content)
				for _, f := range archive.Files {
					targetPath := filepath.ToSlash(filepath.Join("templates", f.Name))
					overlay[targetPath] = &fstest.MapFile{Data: f.Data}
				}
			}
		}
	}

	return &overlayFS{base: baseFS, overlay: overlay}, nil
}

type overlayFS struct {
	base    fs.FS
	overlay fstest.MapFS
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if f, err := o.overlay.Open(name); err == nil {
		return f, nil
	}
	return o.base.Open(name)
}

func (o *overlayFS) ReadDir(name string) ([]fs.DirEntry, error) {
	seen := make(map[string]bool)
	var entries []fs.DirEntry

	if oEntries, err := fs.ReadDir(o.overlay, name); err == nil {
		for _, e := range oEntries {
			entries = append(entries, e)
			seen[e.Name()] = true
		}
	}

	if bEntries, err := fs.ReadDir(o.base, name); err == nil {
		for _, e := range bEntries {
			if !seen[e.Name()] {
				entries = append(entries, e)
			}
		}
	} else if len(entries) == 0 {
		return nil, err
	}

	return entries, nil
}
