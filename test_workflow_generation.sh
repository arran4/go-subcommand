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

# Run gosubc generate-github-workflow
echo "Running gosubc generate-github-workflow..."
go run ./cmd/gosubc generate-github-workflow --dir "$TEMP_DIR"

# Check if file exists
WORKFLOW_FILE="$TEMP_DIR/.github/workflows/generate_verification.yml"
if [ -f "$WORKFLOW_FILE" ]; then
  echo "Workflow file created successfully: $WORKFLOW_FILE"
  # Optional: check content
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

echo "Verification passed."
