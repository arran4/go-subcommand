package main

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"example.com/nocommand/cmd"
)

func TestNoCommandErrors(t *testing.T) {
	for _, tc := range []struct {
		args []string
		kind cmd.UsageErrorKind
	}{
		{nil, cmd.UsageNoCommand},
		{[]string{"--"}, cmd.UsageNoCommand},
		{[]string{"missing"}, cmd.UsageUnknownCommand},
		{[]string{"--", "run"}, cmd.UsageUnknownCommand},
		{[]string{"--unknown"}, cmd.UsageUnknownFlag},
		{[]string{"number"}, cmd.UsageInvalidArguments},
	} {
		root, err := NewRoot("app", "dev", "none", "unknown")
		if err != nil {
			t.Fatal(err)
		}
		err = root.Execute(tc.args)
		wrapped := fmt.Errorf("caller: %w", err)
		var usage *cmd.UsageError
		if !errors.Is(err, cmd.ErrUsage) || !errors.Is(wrapped, cmd.ErrUsage) || !errors.As(wrapped, &usage) || usage.Kind != tc.kind {
			t.Fatalf("%v: lost typed error: %v", tc.args, err)
		}
		if tc.kind == cmd.UsageNoCommand {
			if usage.CommandPath != "app" || usage.Flag != "" || usage.Err != nil {
				t.Fatalf("%#v", usage)
			}
			if generatedExitCode(wrapped) != 1 {
				t.Fatal("no-command default status")
			}
			for _, code := range []int{0, 1, 7} {
				explicit := fmt.Errorf("outer: %w", &cmd.ErrExitCode{Code: code, Err: wrapped})
				if generatedExitCode(explicit) != code {
					t.Fatal("lost explicit exit override")
				}
			}
		}
	}
	var stderr bytes.Buffer
	if status := generatedRun(nil, &stderr); status != 1 || stderr.String() != "Error: no command provided\n" {
		t.Fatalf("status=%d stderr=%q", status, stderr.String())
	}
}
