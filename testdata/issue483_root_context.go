package application

import (
	"fmt"

	"example.com/contextapp/buildinfo"
)

// Root is a subcommand `contextapp`.
//
// Flags:
//
//	buildInfo: (root context) Build metadata supplied by the application constructor
func Root(buildInfo buildinfo.BuildInfo) {}

// Child is a subcommand `contextapp child`.
//
// Flags:
//
//	buildInfo: (from parent) Build metadata from the root constructor
func Child(buildInfo buildinfo.BuildInfo) error {
	if buildInfo.Version != "v1" {
		return fmt.Errorf("child got %q", buildInfo.Version)
	}
	return nil
}

// Grandchild is a subcommand `contextapp child grandchild`.
//
// Flags:
//
//	buildInfo: (from parent) Build metadata from the root constructor
func Grandchild(buildInfo buildinfo.BuildInfo) error {
	if buildInfo.Version != "v1" {
		return fmt.Errorf("grandchild got %q", buildInfo.Version)
	}
	return nil
}
