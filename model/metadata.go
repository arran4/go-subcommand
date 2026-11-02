package model

import "fmt"

// NoCommandPolicy controls a root invocation without a selected command.
type NoCommandPolicy string

const (
	NoCommandDefault NoCommandPolicy = ""
	NoCommandError   NoCommandPolicy = "error"
)

// CommandMetadata describes a command without registering an executable action.
// Metadata may enrich an action discovered independently from Go source.
type CommandMetadata struct {
	Path         string
	Description  string
	ExtendedHelp string
	Aliases      []string
	DefaultRoute string
	NoCommand    NoCommandPolicy
	CLIParser    string
	Filename     string
	Line         int
}

// ManSection is a validated, command-scoped manual appendix.
type ManSection struct {
	Name string
	Text string
}

func metadataError(metadata *CommandMetadata, err error) error {
	if metadata == nil || err == nil {
		return err
	}
	return fmt.Errorf("%s:%d: %w", metadata.Filename, metadata.Line, err)
}
