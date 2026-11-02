package app

import (
	"fmt"
	"io"
	"os"
)

type Resource struct {
	Name string
}

func (r *Resource) Close() error {
	markerFile := os.Getenv("CLEANUP_MARKER_FILE")
	if markerFile != "" {
		f, err := os.OpenFile(markerFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
		if err == nil {
			fmt.Fprintf(f, "close:%s\n", r.Name)
			_ = f.Close()
		}
	}
	if os.Getenv("FAIL_CLOSE_"+r.Name) == "1" {
		return fmt.Errorf("cleanup failed for %s", r.Name)
	}
	return nil
}

func OpenFirst() (io.Closer, error) {
	if os.Getenv("FAIL_ACQUIRE_FIRST") == "1" {
		return nil, fmt.Errorf("failed to acquire first")
	}
	return &Resource{Name: "first"}, nil
}

func OpenSecond(r1 io.Closer) (io.Closer, error) {
	if os.Getenv("FAIL_ACQUIRE_SECOND") == "1" {
		return nil, fmt.Errorf("failed to acquire second")
	}
	return &Resource{Name: "second"}, nil
}

// Run is a subcommand `app run`.
// Flags:
//
//	r1: (generator: "example.com/closable".OpenFirst()) First closable resource
//	r2: (generator: "example.com/closable".OpenSecond(r1)) Second closable resource depending on first
func Run(r1, r2 io.Closer) error {
	if os.Getenv("FAIL_ACTION") == "1" {
		return fmt.Errorf("action error")
	}
	fmt.Println("action executed")
	return nil
}
