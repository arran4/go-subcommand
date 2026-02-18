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
EOF

# Generate code using gosubc
# Assuming gosubc is in the PATH or we build it first.
# The workflow should ensure gosubc is available.
echo "Generating code..."
if command -v gosubc &> /dev/null; then
    gosubc generate --dir "$TEMP_DIR"
else
    go run ./cmd/gosubc generate --dir "$TEMP_DIR"
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

echo "All integration tests passed."
