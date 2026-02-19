#!/bin/bash
set -e

# Build and Install gosubc
echo "Installing gosubc..."
go install ./cmd/gosubc

# Verify installation
if ! command -v gosubc &> /dev/null; then
    echo "Error: gosubc not installed or not in PATH"
    exit 1
fi

echo "gosubc installed successfully at $(which gosubc)"

# Create a temporary directory for integration test
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Using temp dir: $TEMP_DIR"

# Define module name
MODULE_NAME="example.com/integrationtest"

# Create go.mod
cat <<EOF > "$TEMP_DIR/go.mod"
module $MODULE_NAME

go 1.25
EOF

# Create pkg directory
mkdir -p "$TEMP_DIR/pkg"

# Create lib.go with a simple subcommand definition
cat <<EOF > "$TEMP_DIR/pkg/lib.go"
package pkg

import "fmt"

// Hello is a subcommand \`testapp hello\`
// Flags:
//
//	name: --name -n (default: "World") Name to greet
func Hello(name string) {
    fmt.Printf("Hello, %s!\n", name)
}
EOF

# Run gosubc generate
echo "Running gosubc generate..."
gosubc generate --dir "$TEMP_DIR"

# Verify generated code
if [ ! -d "$TEMP_DIR/cmd/testapp" ]; then
    echo "Error: Generated cmd directory not found!"
    exit 1
fi

# Build the generated app
echo "Building generated app..."
go build -o "$TEMP_DIR/testapp" "$TEMP_DIR/cmd/testapp"

# Run the generated app
echo "Running generated app..."
OUTPUT=$("$TEMP_DIR/testapp" hello --name "Integration")
EXPECTED="Hello, Integration!"

if [ "$OUTPUT" != "$EXPECTED" ]; then
    echo "Error: Unexpected output. Expected '$EXPECTED', got '$OUTPUT'"
    exit 1
fi

echo "Integration test passed!"
