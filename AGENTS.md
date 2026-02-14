# Agent Instructions

## Code Generation & Testing
* **Keep README.md and txtar tests up to date:** When adding new features, ensure that the `README.md` documentation and `txtar` tests (in `templates/testdata`) are updated to reflect the changes. This helps maintain a comprehensive regression suite and up-to-date documentation.

## Tooling Generation (Bootstrapping)

The `gosubc` command line tool (`cmd/gosubc`) is self-hosting: it uses the library features to generate its own command structure.

*   **Regenerating `gosubc`:**
    The code in `cmd/gosubc` is generated. To update it (e.g., after modifying templates or the parser), you must run the generator on itself:
    ```bash
    go run ./cmd/gosubc generate
    ```
    This command runs the *current* CLI code to parse the *current* project sources and regenerate the `cmd/gosubc/*.go` files.

*   **Generating Examples:**
    Examples typically include a `//go:generate` directive (e.g., `examples/complex`, `examples/returns`). This directive usually attempts to use a locally installed `gosubc` or falls back to `go run`.

    **Important:** Because examples are standalone modules that may rely on the local root module, it is recommended to use `go work` to ensure `go generate` can locate dependencies correctly without publishing changes.
    ```bash
    go work init . ./examples/returns ./examples/complex
    go generate ./examples/...
    rm go.work go.work.sum # Cleanup
    ```
    *Note: `examples/basic1` may not include this directive and might be maintained manually.*

## Verification

To ensure the integrity of the codebase and generated artifacts, follow these verification steps:

1.  **Run Tests:** Execute the standard Go test suite to verify all packages, including templates and generated code.
    ```bash
    go test ./...
    ```

2.  **Run Linting:** Use `golangci-lint` to check for code style and potential issues. Ensure configuration in `.golangci.yml` excludes `examples` and `testdata` if necessary.
    ```bash
    golangci-lint run
    ```

3.  **Verify Examples:**
    The examples are standalone modules. To verify them, you may need to run tests inside their directories or use a `go.work` file.
    ```bash
    # Option 1: Using go.work (recommended for dev)
    go work init . ./examples/basic1 ./examples/complex ./examples/returns
    go test ./...

    # Option 2: Individual verification
    (cd examples/basic1 && go test ./...)
    (cd examples/complex && go test ./...)
    (cd examples/returns && go test ./...)
    ```

4.  **Regenerate Tooling (Optional):**
    If explicit regeneration verification is needed, refer to the **Tooling Generation** section above.
* `go-subcommand` should never be a dependency of generated code.

## Troubleshooting

*   **golangci-lint Issues:**
    *   If you encounter issues with `golangci-lint` in CI or locally, focus first on the configuration file (`.golangci.yml`) and then on the Go version.
    *   **DO NOT** change the `golangci-lint` version or the `golangci-lint-action` version in workflows, even if they appear to be failing. Downgrading or pinning versions is generally discouraged unless directed otherwise.
    *   Ensure the `.golangci.yml` configuration (if present) is compatible with the version of the linter being used.
