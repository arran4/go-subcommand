#!/bin/bash
set -e

# Assuming gosubc is installed and in PATH
if ! command -v gosubc &> /dev/null; then
    export PATH=$PATH:$(go env GOPATH)/bin
fi

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Using temp dir: $TEMP_DIR"

MODULE_NAME="example.com/usagetest"

cat <<EOF > "$TEMP_DIR/go.mod"
module $MODULE_NAME

go 1.25
EOF

mkdir -p "$TEMP_DIR/pkg"

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

cd "$TEMP_DIR"

gosubc generate

if [ ! -d "cmd/testapp" ]; then
    echo "Error: Generated cmd directory not found!"
    exit 1
fi

go build -o testapp ./cmd/testapp

echo "Running usage test..."
OUTPUT=$(./testapp help 2>&1)
echo "Got output:"
echo "$OUTPUT"

if [[ "$OUTPUT" != *"hello"* ]]; then
    echo "Error: usage output should contain 'hello'"
    exit 1
fi

echo "Running usage recursive test..."
OUTPUT=$(./testapp help -deep 2>&1)
echo "Got output:"
echo "$OUTPUT"

if [[ "$OUTPUT" != *"hello"* ]]; then
    echo "Error: recursive usage output should contain 'hello'"
    exit 1
fi

echo "Usage test passed!"
