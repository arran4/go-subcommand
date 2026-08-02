package go_subcommand

import (
	"bytes"
	"embed"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"golang.org/x/tools/txtar"
)

func splitInputExpected(ar *txtar.Archive) (input, expected fstest.MapFS) {
	input = fstest.MapFS{}
	expected = fstest.MapFS{}

	for _, f := range ar.Files {
		switch {
		case strings.HasPrefix(f.Name, "input/"):
			input[strings.TrimPrefix(f.Name, "input/")] = &fstest.MapFile{Data: f.Data}
		case strings.HasPrefix(f.Name, "expected/"):
			expected[strings.TrimPrefix(f.Name, "expected/")] = &fstest.MapFile{Data: f.Data}
		}
	}
	return input, expected
}

//go:embed testdata/scan/*.txtar
var scanCases embed.FS

func TestScan(t *testing.T) {
	var cases []string
	err := fs.WalkDir(scanCases, "testdata/scan", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".txtar") {
			return nil
		}
		cases = append(cases, p)
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk testdata: %v", err)
	}
	sort.Strings(cases)

	for _, tc := range cases {
		tc := tc
		t.Run(strings.TrimSuffix(path.Base(tc), ".txtar"), func(t *testing.T) {
			raw, err := scanCases.ReadFile(tc)
			if err != nil {
				t.Fatalf("failed to read testcase %s: %v", tc, err)
			}
			ar := txtar.Parse(raw)
			inputFS, expectedFS := splitInputExpected(ar)

			dir := t.TempDir()
			for name, file := range inputFS {
				err := os.MkdirAll(path.Join(dir, path.Dir(name)), 0755)
				if err != nil {
					t.Fatalf("failed to mkdir for %s: %v", name, err)
				}
				err = os.WriteFile(path.Join(dir, name), file.Data, 0644)
				if err != nil {
					t.Fatalf("failed to write %s: %v", name, err)
				}
			}

			// Read options if provided
			var options struct {
				Dir        string   `json:"dir"`
				ParserName string   `json:"parser_name"`
				Paths      []string `json:"paths"`
				Recursive  bool     `json:"recursive"`
			}
			for _, f := range ar.Files {
				if f.Name == "options.json" {
					if err := json.Unmarshal(f.Data, &options); err != nil {
						t.Fatalf("failed to parse options.json: %v", err)
					}
					break
				}
			}

			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			if options.Dir == "" {
				options.Dir = dir
			} else {
				options.Dir = path.Join(dir, options.Dir)
			}
			if options.ParserName == "" {
				options.ParserName = "commentv1"
			}

			err = Scan(options.Dir, options.ParserName, options.Paths, options.Recursive)

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = io.Copy(&buf, r)
			output := buf.String()

			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}

			expectedOut, ok := expectedFS["output.txt"]
			if ok {
				expectedStr := string(expectedOut.Data)
				if strings.TrimSpace(output) != strings.TrimSpace(expectedStr) {
					t.Errorf("Expected output:\n%s\nGot:\n%s", expectedStr, output)
				}
			}
		})
	}
}

func TestScan_Error(t *testing.T) {
	err := Scan(".", "unknown_parser", nil, false)
	if err == nil {
		t.Errorf("Expected error with unknown parser, got nil")
	}
}
