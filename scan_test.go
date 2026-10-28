package go_subcommand

import (
	"bytes"
	"embed"
	"encoding/json"
	"io/fs"
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

			if options.Dir == "" {
				options.Dir = "."
			}
			if options.ParserName == "" {
				options.ParserName = "commentv1"
			}
			var buf bytes.Buffer
			testFS := fs.FS(inputFS)
			if options.Dir != "." {
				testFS, err = fs.Sub(inputFS, options.Dir)
				if err != nil {
					t.Fatalf("sub filesystem %s: %v", options.Dir, err)
				}
			}
			err = ScanWithFS(testFS, &buf, options.ParserName, options.Paths, options.Recursive)
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
	err := ScanWithFS(fstest.MapFS{}, &bytes.Buffer{}, "unknown_parser", nil, false)
	if err == nil {
		t.Errorf("Expected error with unknown parser, got nil")
	}
}
