package main

import (
	"errors"
	"io"
	"strings"
	"testing"
)

type mockReadCloser struct {
	io.Reader
	CloseFunc func() error
	CloseCount int
}

func (m *mockReadCloser) Close() error {
	m.CloseCount++
	if m.CloseFunc != nil {
		return m.CloseFunc()
	}
	return nil
}

func TestIssue455_Cleanup(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		actionErr     error
		cleanErrs     []error
		expectErrText string
		expectOrder   []string
	}{
		{
			name: "success + successful cleanup",
			args: []string{"--in", "in1.txt", "--in", "in2.txt"},
			expectOrder: []string{"in2.txt", "in1.txt"},
		},
		{
			name:      "success + cleanup failure",
			args:      []string{"--in", "in1.txt", "--in", "in2.txt"},
			cleanErrs: []error{errors.New("cleanup error 1"), errors.New("cleanup error 2")},
			expectErrText: "cleanup error 1", // First error encountered during reverse execution
			expectOrder: []string{"in2.txt", "in1.txt"},
		},
		{
			name:      "action error + cleanup failure preserves action error",
			args:      []string{"--in", "in1.txt", "--in", "in2.txt"},
			actionErr: errors.New("action error"),
			cleanErrs: []error{errors.New("cleanup error")},
			expectErrText: "app failed: action error",
			expectOrder: []string{"in2.txt", "in1.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, _ := NewRoot("app", "test", "test", "test")

			// Track cleanups
			var cleanupsRun []string
			var currentMockReaders []*mockReadCloser

			// Override generatedOpenReader
			origOpenReader := generatedOpenReader
			generatedOpenReader = func(name string) (io.ReadCloser, error) {
				m := &mockReadCloser{
					Reader: strings.NewReader("dummy"),
					CloseFunc: func() error {
						cleanupsRun = append(cleanupsRun, name)
						if len(tt.cleanErrs) > 0 {
							err := tt.cleanErrs[0]
							tt.cleanErrs = tt.cleanErrs[1:]
							return err
						}
						return nil
					},
				}
				currentMockReaders = append(currentMockReaders, m)
				return m, nil
			}

			origCommandAction := root.CommandAction
			root.CommandAction = func(c *RootCmd) error {
				return tt.actionErr
			}

			defer func() {
				generatedOpenReader = origOpenReader
				root.CommandAction = origCommandAction
			}()

			err := root.Execute(tt.args)

			if tt.expectErrText == "" {
				if err != nil {
					t.Errorf("expected nil error, got: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.expectErrText) {
					t.Errorf("expected error containing %q, got: %v", tt.expectErrText, err)
				}
			}

			for _, m := range currentMockReaders {
				if m.CloseCount != 1 {
					t.Errorf("expected all resources to be closed exactly once, got %d for one resource", m.CloseCount)
				}
			}

			for i, v := range tt.expectOrder {
			    if i >= len(cleanupsRun) || cleanupsRun[i] != v {
			        t.Errorf("expected cleanup order %v, got %v", tt.expectOrder, cleanupsRun)
			    }
			}
		})
	}
}

func TestIssue455_PartialAcquisition(t *testing.T) {
	root, _ := NewRoot("app", "test", "test", "test")

	var cleanupsRun []string
	var currentMockReaders []*mockReadCloser

	origOpenReader := generatedOpenReader
	defer func() { generatedOpenReader = origOpenReader }()

	generatedOpenReader = func(name string) (io.ReadCloser, error) {
		if name == "fail.txt" {
			return nil, errors.New("open error")
		}
		m := &mockReadCloser{
			Reader: strings.NewReader("dummy"),
			CloseFunc: func() error {
				cleanupsRun = append(cleanupsRun, name)
				return nil
			},
		}
		currentMockReaders = append(currentMockReaders, m)
		return m, nil
	}

	err := root.Execute([]string{"--in", "success1.txt", "--in", "fail.txt"})

	if err == nil || !strings.Contains(err.Error(), "open error") {
		t.Fatalf("expected open error, got %v", err)
	}

	if len(cleanupsRun) != 1 || cleanupsRun[0] != "success1.txt" {
		t.Fatalf("expected success1.txt to be cleaned up, got %v", cleanupsRun)
	}

	for _, m := range currentMockReaders {
		if m.CloseCount != 1 {
			t.Errorf("expected closed exactly once, got %d", m.CloseCount)
		}
	}
}

func TestIssue455_BorrowedStdinNotClosed(t *testing.T) {
	root, _ := NewRoot("app", "test", "test", "test")

	origOpenReader := generatedOpenReader
	defer func() { generatedOpenReader = origOpenReader }()

	calledOpen := false
	generatedOpenReader = func(name string) (io.ReadCloser, error) {
		calledOpen = true
		return nil, errors.New("should not be called for stdin")
	}

	err := root.Execute([]string{"--in", "-"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledOpen {
		t.Fatalf("generatedOpenReader was called for '-'")
	}
}
