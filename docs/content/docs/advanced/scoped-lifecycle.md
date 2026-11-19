---
title: "Scoped providers and command initializers"
weight: 14
---

# Scoped providers and command initializers

> This guide documents the scoped lifecycle feature merged in [PR #555](https://github.com/arran4/go-subcommand/pull/555) ([issue #522](https://github.com/arran4/go-subcommand/issues/522)). It is **on main but not part of v0.0.32**. The unified `Requires:` declaration is a separate, not-yet-implemented improvement tracked in [#561](https://github.com/arran4/go-subcommand/issues/561).

Use the existing [functionless command metadata](../command-metadata/) to attach initialization to a command subtree without inventing a dummy action. Two behaviors are intentionally different:

| Capability | Trigger | Work done |
| --- | --- | --- |
| **Mandatory initializer** | Every selected executable descendant of its declaring command | Runs even when the action has no injected provider |
| **Lazy shared provider** | First request for its typed value on the selected invocation | May never run; caches the value **and error** for that invocation |

Both use the existing typed provider dependency model. Neither means "initialize the entire process", and neither introduces a global singleton or a dependency on `go-subcommand` in generated applications.

## Example: an admin subtree

This is the source from the compiled lifecycle fixture (module `example.com/lifecycleapp`, with `go 1.22`). The functions simulate authorization and resource failures with environment variables **for testing only**, not production authentication.

```go
// session/session.go
package session

import (
	"fmt"
	"os"
)

type AdminSession struct {
	User string
}

func (s *AdminSession) Close() error {
	fmt.Printf("close:session:%s\n", s.User)
	if os.Getenv("FAIL_CLOSE") == "1" {
		return fmt.Errorf("cleanup failure")
	}
	return nil
}

func RequireAdmin() error {
	fmt.Println("init:require-admin")
	if os.Getenv("FAIL_ADMIN") == "1" {
		return fmt.Errorf("admin authorization failed")
	}
	return nil
}

func OpenAdminSession() (*AdminSession, error) {
	fmt.Println("provider:open-session")
	if os.Getenv("FAIL_SESSION") == "1" {
		return nil, fmt.Errorf("session connection failed")
	}
	return &AdminSession{User: "admin-user"}, nil
}
```

Declare ordinary action functions and the metadata-only parent:

```go
// app.go
package app

import (
	"fmt"

	"example.com/lifecycleapp/session"
)

// Root is a subcommand `app` -- Application root.
func Root() error {
	fmt.Println("action:root")
	return nil
}

// Admin is a command `app admin` -- Administrative commands.
// Initializer: "example.com/lifecycleapp/session".RequireAdmin
// Shared provider: session = "example.com/lifecycleapp/session".OpenAdminSession

// List is a subcommand `app admin list` -- List resources.
func List(session *session.AdminSession) error {
	fmt.Printf("action:list:%s\n", session.User)
	return nil
}

// Delete is a subcommand `app admin delete` -- Delete resources.
//
// Flags:
//
//	id: @1 Resource id
func Delete(session *session.AdminSession, id string) error {
	fmt.Printf("action:delete:%s:%s\n", id, session.User)
	return nil
}

// Audit is a subcommand `app admin audit` -- Audit resources.
//
// Flags:
//
//	dryRun: --dry-run Dry run mode
func Audit(session func() (*session.AdminSession, error), dryRun bool) error {
	if dryRun {
		fmt.Println("action:audit:dry-run")
		return nil
	}
	s, err := session()
	if err != nil {
		return err
	}
	fmt.Printf("action:audit:live:%s\n", s.User)
	return nil
}

// About is a subcommand `app admin about` -- About admin module.
func About() error {
	fmt.Println("action:about")
	return nil
}
```

Adjust `example.com/lifecycleapp` to your module path. The comment on `Admin` declares a **structural command only**, not a Go function. In the merged syntax, keep the explicit provider binding name `session` aligned with the consuming parameter, as in this tested example. It is not a CLI flag. Do not assume arbitrary parameter names are currently inferred from return types; that improvement is tracked in #561.

The tested implementation includes the following behaviors:

| Invocation | Mandatory guard | Session provider |
| --- | --- | --- |
| `app admin list` | Runs | Acquired before the list action |
| `app admin about` | Runs | **Not** acquired |
| `app admin audit --dry-run` | Runs | **Not** acquired if callback unused |
| `app admin audit` | Runs | Acquired on callback invocation |
| `app admin --help` | **Not** run | **Not** acquired |
| `app help admin` | **Not** run | **Not** acquired |

The exact executable fixture is [`issue522_lifecycle.txtar`](https://github.com/arran4/go-subcommand/blob/main/templates/testdata/runtime/issue522_lifecycle.txtar); check the fixture and final merged code when upgrading.

## Native command configuration

Equivalent attachments can be declared in an optional `.mdc` or `.mdconf` configuration using the existing `# Commands` section:

~~~text
format-version: 1

# Commands

```command: app admin
Description: Administrative commands.
Initializer: "example.com/lifecycleapp/session".RequireAdmin
Shared provider: session = "example.com/lifecycleapp/session".OpenAdminSession
```
~~~

Do not declare the same canonical `app admin` metadata independently in both source and config: command metadata identity and duplication rules still apply. The initializer does not turn a functionless parent into an action.

## Scope, laziness and cleanup

- **Selected-path execution:** A mandatory root or parent guard runs root-to-leaf before its selected executable descendant action, after argument binding and validation. An unrelated sibling does not inherit the guard.
- **Shared dependency identity:** The generator may store a memoized typed provider accessor on the lowest safe common ancestor used by multiple descendants. Hoisting storage **does not** make acquisition eager. Use explicit binding declarations for potentially ambiguous scopes and inputs; stronger typed identity and dependency ordering are tracked in #561.
- **Ordinary injection:** A typed argument such as `func List(session *session.AdminSession) error` obtains the provider before calling the action.
- **Opt-in delayed acquisition:** A typed callback such as `func Audit(session func() (*session.AdminSession, error), dryRun bool) error` can request the dependency inside the action, or never request it.
- **Once per invocation:** A provider's value and any initialization failure are memoized for the current command execution. Reusing a command object must start a fresh invocation rather than retaining closed resources from a previous invocation.
- **Resource lifecycle:** An acquired resource implementing `io.Closer` joins invocation-scoped cleanup. Close in reverse actual acquisition order, including partial failure paths; no cleanup for resources that were never acquired.
- **Errors:** An initializer error aborts before the action. Preserve `errors.Is` and `errors.As` as errors travel through generated command execution. Cleanup errors use the generator's existing error policy.
- **Help/version:** No business initialization solely to render built-in help, usage, version or a failed parse. Avoid side effects when no executable action is selected.

The generated command tree still owns parsing and dispatch; application packages remain independent of generated `package main` code. Use ordinary Go function signatures and comments, rather than manually registering callbacks or importing generated root types.

## Validation and compatibility

Use only the tested declaration and binding forms above. More comprehensive type resolution, cross-requirement ordering and ambiguity diagnostics are tracked in #561. Do not work around unresolved generator errors by manually editing generated source.

The generator's toolchain version and **generated application's Go minimum** are separate constraints: verify emitted source with an actual Go 1.22 compiler and `GOTOOLCHAIN=local`. Generated applications must not import the generator at runtime.

For detailed lifecycle behavior, review the source and compiled invocations in [PR #555](https://github.com/arran4/go-subcommand/pull/555). This guide uses the source from the merged fixture rather than speculative syntax.
