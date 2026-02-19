#!/bin/bash
set -e

# Create a temporary directory
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "$TEMP_DIR"' EXIT

echo "Using temp dir: $TEMP_DIR"

# Create go.mod
cat <<EOF > "$TEMP_DIR/go.mod"
module example.com/test
go 1.25
EOF

# Test 1: Run gosubc init --gh-verification
echo "--- Test 1: gosubc init --gh-verification ---"
go run ./cmd/gosubc init --dir "$TEMP_DIR" --gh-verification

# Check if file exists
WORKFLOW_FILE="$TEMP_DIR/.github/workflows/generate_verification.yml"
if [ -f "$WORKFLOW_FILE" ]; then
  echo "Workflow file created successfully: $WORKFLOW_FILE"
  if grep -q "Verify Generation" "$WORKFLOW_FILE"; then
    echo "Content verification passed."
  else
    echo "Content verification failed."
    exit 1
  fi
else
  echo "Workflow file NOT created."
  exit 1
fi

# Clean up for next test
rm -rf "$TEMP_DIR/.github"

# Test 2: Run gosubc generate gh-verification
echo "--- Test 2: gosubc generate gh-verification ---"
go run ./cmd/gosubc generate gh-verification --dir "$TEMP_DIR"

# Check if file exists
if [ -f "$WORKFLOW_FILE" ]; then
  echo "Workflow file created successfully via subcommand: $WORKFLOW_FILE"
  if grep -q "Verify Generation" "$WORKFLOW_FILE"; then
    echo "Content verification passed."
  else
    echo "Content verification failed."
    exit 1
  fi
else
  echo "Workflow file NOT created via subcommand."
  exit 1
fi

echo "All verifications passed."
