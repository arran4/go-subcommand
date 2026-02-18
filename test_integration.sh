#!/bin/bash
set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Using temp dir: $TEMP_DIR"

# Define module name
MODULE_NAME="example.com/integration"

# Create go.mod
cat <<EOF > "$TEMP_DIR/go.mod"
module $MODULE_NAME

go 1.25
EOF

# Create pkg directory
mkdir -p "$TEMP_DIR/pkg"

# Create lib.go with the subcommand definition
cat <<EOF > "$TEMP_DIR/pkg/lib.go"
package pkg

import "fmt"

// Run is a subcommand \`testapp run\`
//
// Flags:
//	output: -o, --output string Output file (default: "default")
//
// input: @1
func Run(output string, input string) {
	fmt.Printf("Output: %q, Input: %q\n", output, input)
}

// Issue78 is a subcommand \`testapp issue78\`
//
// Flags:
//	myflag: -f, --flag string Description (default: "default_val")
//
func Issue78(myflag string) {
	fmt.Printf("myflag: %q\n", myflag)
}
EOF

# Generate code using gosubc
# Assuming gosubc is in the PATH or we build it first.
# The workflow should ensure gosubc is available.
echo "Generating code..."
if command -v gosubc &> /dev/null; then
    gosubc generate --dir "$TEMP_DIR"
else
    # Fallback to running from source if gosubc is not in PATH
    # Assuming we are running from the repo root
    if [ -f "./go.mod" ]; then
        go run ./cmd/gosubc generate --dir "$TEMP_DIR"
    else
        echo "Error: gosubc not found and not in repo root."
        exit 1
    fi
fi

# Tidy dependencies
echo "Running go mod tidy..."
(cd "$TEMP_DIR" && go mod tidy)

# Build the generated application
echo "Building testapp..."
(cd "$TEMP_DIR" && go build -o testapp ./cmd/testapp)

# Test 1: Run with '-' argument
echo "Running testapp run - ..."
OUTPUT=$("$TEMP_DIR/testapp" run -)
EXPECTED='Output: "default", Input: "-"'

if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 1 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 1 Passed."

# Test 2: Run with --output and '-' argument
echo "Running testapp run --output out.txt - ..."
OUTPUT=$("$TEMP_DIR/testapp" run --output out.txt -)
EXPECTED='Output: "out.txt", Input: "-"'

if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 2 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 2 Passed."


# Test 3: Issue 78 - Run with no flags -> Default value
echo "Running testapp issue78 (default)..."
OUTPUT=$("$TEMP_DIR/testapp" issue78)
EXPECTED='myflag: "default_val"'
if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 3 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 3 Passed."

# Test 4: Issue 78 - Run with short alias -f -> Update value
echo "Running testapp issue78 -f user_val..."
OUTPUT=$("$TEMP_DIR/testapp" issue78 -f user_val)
EXPECTED='myflag: "user_val"'
if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 4 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 4 Passed."

# Test 5: Issue 78 - Run with long alias --flag -> Update value
echo "Running testapp issue78 --flag user_val..."
OUTPUT=$("$TEMP_DIR/testapp" issue78 --flag user_val)
EXPECTED='myflag: "user_val"'
if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 5 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 5 Passed."

# Test 6: Issue 78 - Run with both (last wins) -> Update value
echo "Running testapp issue78 -f v1 --flag v2..."
OUTPUT=$("$TEMP_DIR/testapp" issue78 -f v1 --flag v2)
EXPECTED='myflag: "v2"'
if [[ "$OUTPUT" != *"$EXPECTED"* ]]; then
  echo "Test 6 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  exit 1
fi
echo "Test 6 Passed."

# Test 7: Programmatic Override
# Create a custom main file in cmd/testapp/main_override.go
cat <<EOF > "$TEMP_DIR/cmd/testapp/main_override.go"
package main

import (
	"fmt"
	"os"
)

func main() {
	// RootCmd is generated in root.go
	root, err := NewRoot("testapp", "v0.0.1", "none", "none")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating root: %v\n", err)
		os.Exit(1)
	}
	// NewIssue78 returns *Issue78 struct. Field myflag corresponds to parameter myflag.
	cmd := root.NewIssue78()
	cmd.myflag = "programmatic_override"

	// Execute with empty args, should use the preset value
	if err := cmd.Execute([]string{}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
EOF

echo "Running programmatic override test..."
# Rename generated main.go to avoid conflict
mv "$TEMP_DIR/cmd/testapp/main.go" "$TEMP_DIR/cmd/testapp/main_orig.go.bak"

# Run go run . in cmd/testapp
(cd "$TEMP_DIR/cmd/testapp" && go run .) > "$TEMP_DIR/prog_output.txt" 2>&1 || true
PROG_OUTPUT=$(cat "$TEMP_DIR/prog_output.txt")

EXPECTED_PROG='myflag: "programmatic_override"'
if [[ "$PROG_OUTPUT" != *"$EXPECTED_PROG"* ]]; then
  echo "Test 7 Failed (Programmatic Override)!"
  echo "Expected output to contain: $EXPECTED_PROG"
  echo "Got: $PROG_OUTPUT"
  exit 1
fi
echo "Test 7 Passed (Programmatic Override)."


echo "All integration tests passed."
