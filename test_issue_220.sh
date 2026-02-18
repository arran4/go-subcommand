#!/bin/bash
set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Using temp dir: $TEMP_DIR"

# Define module name
MODULE_NAME="example.com/issue220"

# Create go.mod
cat <<EOF > "$TEMP_DIR/go.mod"
module $MODULE_NAME

go 1.25
EOF

mkdir -p "$TEMP_DIR/pkg"

# Create lib.go with the subcommand definition
cat <<EOF > "$TEMP_DIR/pkg/lib.go"
package pkg

import "fmt"

// Run is a subcommand \`testapp run\`
//
// Flags:
//	verbose: -v string Verbose level
//	x: -x bool X flag
//	y: -y bool Y flag
//	z: -z bool Z flag
//
func Run(verbose string, x bool, y bool, z bool) {
	fmt.Printf("Verbose: %q, X: %v, Y: %v, Z: %v\n", verbose, x, y, z)
}
EOF

# Determine gosubc command
if command -v gosubc &> /dev/null; then
    GOSUBC="gosubc"
else
    GOSUBC="go run ./cmd/gosubc"
fi

# Generate code using gosubc
echo "Generating code..."
$GOSUBC generate --dir "$TEMP_DIR"

# Tidy dependencies
echo "Running go mod tidy..."
(cd "$TEMP_DIR" && go mod tidy)

# Build the generated application
echo "Building testapp..."
(cd "$TEMP_DIR" && go build -o testapp ./cmd/testapp)

FAILED=0

# Test 1: Short flag with value attached (-v123)
echo "Test 1: Running testapp run -v123 ..."
OUTPUT=$("$TEMP_DIR/testapp" run -v123 2>&1 || true)
EXPECTED='Verbose: "123"'

if [[ "$OUTPUT" == *"$EXPECTED"* ]]; then
  echo "Test 1 Passed."
else
  echo "Test 1 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  FAILED=1
fi

# Test 2: Bundled boolean flags (-xyz)
echo "Test 2: Running testapp run -xyz ..."
OUTPUT=$("$TEMP_DIR/testapp" run -xyz 2>&1 || true)
EXPECTED='X: true, Y: true, Z: true'

if [[ "$OUTPUT" == *"$EXPECTED"* ]]; then
  echo "Test 2 Passed."
else
  echo "Test 2 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  FAILED=1
fi

# Test 3: Bundled boolean flags with value flag (-xyzv123)
echo "Test 3: Running testapp run -xyzv123 ..."
OUTPUT=$("$TEMP_DIR/testapp" run -xyzv123 2>&1 || true)
EXPECTED='Verbose: "123", X: true, Y: true, Z: true'

if [[ "$OUTPUT" == *"$EXPECTED"* ]]; then
  echo "Test 3 Passed."
else
  echo "Test 3 Failed!"
  echo "Expected output to contain: $EXPECTED"
  echo "Got: $OUTPUT"
  FAILED=1
fi

if [ $FAILED -ne 0 ]; then
    echo "Some tests failed."
    exit 1
fi

echo "All Issue 220 tests passed."
