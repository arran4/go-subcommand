package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"

	app "example.com/typederrors"
	"example.com/typederrors/cmd"
)

func TestTypedUsageErrors(t *testing.T) {
	for _, tc := range []struct {
		args       []string
		kind       cmd.UsageErrorKind
		path, flag string
		cause      bool
	}{
		{[]string{"--unknown"}, cmd.UsageUnknownFlag, "app", "unknown", false},
		{[]string{"group", "missing"}, cmd.UsageUnknownCommand, "app group", "", false},
		{[]string{"help", "group", "missing"}, cmd.UsageUnknownCommand, "app group", "", false},
		{[]string{"group", "check", "--count"}, cmd.UsageMissingFlagValue, "app group check", "count", false},
		{[]string{"group", "check", "--count=bad"}, cmd.UsageInvalidValue, "app group check", "count", true},
		{[]string{"group", "check", "-n", "bad"}, cmd.UsageInvalidValue, "app group check", "n", true},
		{[]string{"group", "check", "--custom=bad"}, cmd.UsageInvalidValue, "app group check", "custom", true},
		{[]string{"required"}, cmd.UsageMissingRequiredFlag, "app required", "token", false},
		{[]string{"positional"}, cmd.UsageInvalidArguments, "app positional", "", false},
		{[]string{"positional", "bad"}, cmd.UsageInvalidValue, "app positional", "", true},
		{[]string{"tail"}, cmd.UsageInvalidArguments, "app tail", "", false},
	} {
		root, err := NewRoot("app", "dev", "none", "unknown")
		if err != nil {
			t.Fatal(err)
		}
		err = root.Execute(tc.args)
		wrapped := fmt.Errorf("caller: %w", err)
		var usage *cmd.UsageError
		if !errors.Is(err, cmd.ErrUsage) || !errors.Is(wrapped, cmd.ErrUsage) || !errors.As(wrapped, &usage) {
			t.Fatalf("%v: unclassified error %v", tc.args, err)
		}
		if usage.Kind != tc.kind || usage.CommandPath != tc.path || usage.Flag != tc.flag || (usage.Err != nil) != tc.cause {
			t.Errorf("%v: %#v", tc.args, usage)
		}
		if tc.flag == "custom" && !errors.Is(err, app.ParserCause) {
			t.Errorf("lost custom parser cause: %v", err)
		}
	}
	root, _ := NewRoot("app", "dev", "none", "unknown")
	if err := root.Execute([]string{"action", "--mode=wrapped-usage"}); err != app.WrappedUsage {
		t.Fatalf("dispatcher replaced application error: %v", err)
	}
	usage := &cmd.UsageError{}
	if !errors.Is(usage, cmd.ErrUsage) || errors.Unwrap(usage) != nil || usage.Error() == "" {
		t.Fatal("nil-cause usage contract")
	}
	root, _ = NewRoot("app", "dev", "none", "unknown")
	readError := root.Execute([]string{"read", "--input", "missing-input.txt"})
	var pathError *os.PathError
	if errors.Is(readError, cmd.ErrUsage) || !errors.As(readError, &pathError) {
		t.Fatalf("I/O error lost its classification: %v", readError)
	}
}

func TestGeneratedProcessErrors(t *testing.T) {
	for _, code := range []int{0, 1, 7} {
		for _, cause := range []error{nil, errors.New("cause"), cmd.ErrUsage} {
			err := &cmd.ErrExitCode{Code: code, Err: cause}
			for _, wrapped := range []error{err, fmt.Errorf("outer: %w", err)} {
				if status := generatedExitCode(wrapped); status != code {
					t.Errorf("status=%d want %d", status, code)
				}
				var output bytes.Buffer
				generatedRenderError(&output, wrapped)
				want := ""
				if cause != nil {
					want = "Error: " + cause.Error() + "\n"
				}
				if output.String() != want {
					t.Errorf("output=%q want %q", output.String(), want)
				}
			}
		}
	}
	if generatedExitCode(nil) != 0 || generatedExitCode(&cmd.UsageError{}) != 1 || generatedExitCode(errors.New("ordinary")) != 1 {
		t.Fatal("default status compatibility")
	}
	var output bytes.Buffer
	generatedRenderError(&output, nil)
	if output.Len() != 0 {
		t.Fatal("nil error was rendered")
	}
}

func TestGoFlagGrammarParity(t *testing.T) {
	// Compare successful parsing and failure precedence with the standard library.
	for _, args := range [][]string{
		{}, {"--count=3"}, {"-count", "-2"}, {"--enabled=false"},
		{"--label", "--"}, {"--", "--unknown"}, {"-", "--unknown"},
		{"positional", "--count=2"}, {"--count=bad", "--unknown"},
		{"--unknown", "--count=bad"}, {"-count"}, {"---count"}, {"-=bad"},
		{"-help"}, {"--label="}, {"--count=1", "--count=2"},
	} {
		makeSet := func() (*flag.FlagSet, *int, *bool, *string) {
			fs := flag.NewFlagSet("parity", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			fs.Usage = func() {}
			return fs, fs.Int("count", 0, ""), fs.Bool("enabled", false, ""), fs.String("label", "", "")
		}
		standard, count, enabled, label := makeSet()
		standardError := standard.Parse(args)
		generated, gotCount, gotEnabled, gotLabel := makeSet()
		remaining, _, generatedError := parseGoFlags(generated, args, "parity")
		if (standardError != nil) != (generatedError != nil) || *count != *gotCount || *enabled != *gotEnabled || *label != *gotLabel {
			t.Fatalf("%v: incompatible values or failure: std=%v generated=%v", args, standardError, generatedError)
		}
		if standardError == nil {
			if len(standard.Args()) != len(remaining) || (len(remaining) > 0 && !reflect.DeepEqual(standard.Args(), remaining)) {
				t.Errorf("%v: remaining=%v want %v", args, remaining, standard.Args())
			}
		} else if standardError.Error() != generatedError.Error() {
			t.Errorf("%v: error=%q want %q", args, generatedError, standardError)
		}
	}
}
