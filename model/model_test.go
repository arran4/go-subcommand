package model

import (
	"testing"
)

func TestValidateParameters_File(t *testing.T) {
	cmdName := "test"
	params := []*FunctionParameter{
		{
			Name: "myFile",
			Type: "*os.File",
		},
	}
	err := validateParameters(params, cmdName)
	if err == nil {
		t.Fatalf("expected error for *os.File parameter, got nil")
	}
	expectedMsg := `parameter "myFile" has type *os.File but no implicit file access mode; use io.Reader/io.ReadCloser for input, io.Writer/io.WriteCloser for output, or configure an explicit provider`
	if err.Error() != expectedMsg {
		t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
	}
}
