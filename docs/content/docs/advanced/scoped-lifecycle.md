---
title: "Scoped providers and command initializers"
weight: 14
---

# Scoped providers and command initializers

> **Since version:** v0.0.34 for `Requires:`; v0.0.33 for legacy `Initializer:` and `Shared provider:` declarations. Prefer `Requires:` for new commands. The earlier forms remain supported.

Use the existing [functionless command metadata](../command-metadata/) to attach initialization to a command subtree without inventing a dummy action. Two behaviors are intentionally different:

| Capability | Trigger | Work done |
| --- | --- | --- |
| **Mandatory initializer** | Every selected executable descendant of its declaring command | Runs even when the action has no injected provider |
| **Lazy shared provider** | First request for its typed value on the selected invocation | May never run; caches the value **and error** for that invocation |

Both use the existing typed provider dependency model. Neither means "initialize the entire process", and neither introduces a global singleton or a dependency on `go-subcommand` in generated applications.

## Canonical `Requires:` declarations

**Since version:** v0.0.34.

A requirement returning only `error` is mandatory for the selected executable path. A requirement returning `T` or `(T, error)` supplies a demand-driven typed value. A value-producing requirement marked `(always)` is mandatory even if no action consumes its result. Multiple directives and top-level comma-separated entries accumulate. Unique exact resolved Go types allow inference; use an explicit `(provider: ...)` binding when the type alone is ambiguous.

### Example: authorization only when needed

Create the module and package directory:

```sh
mkdir requiresapp
cd requiresapp
go mod init example.com/requiresapp
go mod edit -go=1.22
mkdir auth
```

Save the following as `auth/auth.go`. These functions simulate authentication for illustration; they are not production authentication.

```go
package auth

import (
	"fmt"
	"os"
)

type User struct {
	Role string
}

type Audit struct {
	Tag string
}

func CurrentUser() (*User, error) {
	if os.Getenv("FAIL_USER") == "1" {
		fmt.Println("auth:current-user:fail")
		return nil, fmt.Errorf("user lookup failed")
	}
	role := "admin"
	if r := os.Getenv("USER_ROLE"); r != "" {
		role = r
	}
	fmt.Println("auth:current-user")
	return &User{Role: role}, nil
}

func RequireAdmin(user *User) error {
	fmt.Printf("auth:require-admin:%s\n", user.Role)
	if os.Getenv("FAIL_ADMIN") == "1" || user.Role != "admin" {
		return fmt.Errorf("admin authorization failed")
	}
	return nil
}

func AuditTrail() (*Audit, error) {
	fmt.Println("auth:audit-trail")
	if os.Getenv("FAIL_AUDIT") == "1" {
		return nil, fmt.Errorf("audit failed")
	}
	return &Audit{Tag: "audit-1"}, nil
}
```

Save the following as `app.go`:

```go
package app

import (
	"fmt"
	"example.com/requiresapp/auth"
)

type UserAlias = auth.User

// API is a command `app api` -- API endpoints.
// Requires: auth.CurrentUser

// Health is a subcommand `app api health` -- Health check without user acquisition.
func Health() error {
	fmt.Println("action:health")
	return nil
}

// Profile is a subcommand `app api profile` -- Profile using type alias without auth import.
func Profile(user *UserAlias) error {
	fmt.Printf("action:profile:%s\n", user.Role)
	return nil
}

// Users is a command `app api users` -- User management.
// Requires: auth.RequireAdmin

// UsersList is a subcommand `app api users list` -- List users.
func UsersList(user *auth.User) error {
	fmt.Printf("action:users-list:%s\n", user.Role)
	return nil
}

// UsersDelete is a subcommand `app api users delete` -- Delete user via type alias.
func UsersDelete(user *UserAlias) error {
	fmt.Printf("action:users-delete:%s\n", user.Role)
	return nil
}
```

Generate and execute:

```sh
go run github.com/arran4/go-subcommand/cmd/gosubc@v0.0.34 generate --emit-go-generate=false
go run ./cmd/app api health
go run ./cmd/app api users list
```

Expected stdout for health:

```text
action:health
```

Expected stdout for list:

```text
auth:current-user
auth:require-admin:admin
action:users-list:admin
```

A mandatory guard attached directly to `api` would also apply to health. Here it belongs to `api users`; both its dependency and the action receive the same cached user. With `USER_ROLE=guest`, the guard returns an authorization error before the action executes.

## Compatibility example: an admin subtree

Create a separate module `example.com/lifecycleapp` with `go 1.22` and a `session/` directory. The functions simulate authorization and resource failures with environment variables **for testing only**, not production authentication.

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

Adjust `example.com/lifecycleapp` to your module path. The comment on `Admin` declares a **structural command only**, not a Go function. In this legacy example, the explicit provider binding name `session` aligns with the consuming parameter. It is not a CLI flag. The unified resolver preserves named legacy binding while also supporting unique exact-type inference for `Requires:`.

The tested implementation includes the following behaviors:

| Invocation | Mandatory guard | Session provider |
| --- | --- | --- |
| `app admin list` | Runs | Acquired before the list action |
| `app admin about` | Runs | **Not** acquired |
| `app admin audit --dry-run` | Runs | **Not** acquired if callback unused |
| `app admin audit` | Runs | Acquired on callback invocation |
| `app admin --help` | **Not** run | **Not** acquired |
| `app help admin` | **Not** run | **Not** acquired |

To run the legacy example, create a separate module named `example.com/lifecycleapp`, put the two files at the paths shown, and generate with `gosubc generate`. The table describes expected execution.

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

- **Selected-path execution:** A mandatory root or parent guard runs before its selected executable descendant action, after argument binding and validation. Dependencies execute before their consumers; declaration order breaks ties between independent mandatory steps. An unrelated sibling does not inherit the guard.
- **Shared dependency identity:** The generator may store a memoized typed provider accessor on the lowest safe common ancestor used by multiple descendants. Hoisting storage **does not** make acquisition eager. Use explicit bindings for ambiguous types and inputs; the resolver distinguishes provider identity by resolved function, type and scoped input bindings.
- **Ordinary injection:** A typed argument such as `func List(session *session.AdminSession) error` obtains the provider before calling the action.
- **Opt-in delayed acquisition:** A typed callback such as `func Audit(session func() (*session.AdminSession, error), dryRun bool) error` can request the dependency inside the action, or never request it.
- **Once per invocation:** A provider's value and any initialization failure are memoized for the current command execution. Reusing a command object must start a fresh invocation rather than retaining closed resources from a previous invocation.
- **Resource lifecycle:** An acquired resource implementing `io.Closer` joins invocation-scoped cleanup. Close in reverse actual acquisition order, including partial failure paths; no cleanup for resources that were never acquired.
- **Errors:** An initializer error aborts before the action. Preserve `errors.Is` and `errors.As` as errors travel through generated command execution. Cleanup errors use the generator's existing error policy.
- **Help/version:** No business initialization solely to render built-in help, usage, version or a failed parse. Avoid side effects when no executable action is selected.

The generated command tree still owns parsing and dispatch; application packages remain independent of generated `package main` code. Use ordinary Go function signatures and comments, rather than manually registering callbacks or importing generated root types.

## Validation and compatibility

Use the tested declaration and binding forms above. The unified resolver supports typed dependency ordering and ambiguity diagnostics. Do not work around unresolved generator errors by manually editing generated source.

The generator's toolchain version and **generated application's Go minimum** are separate constraints: verify emitted source with an actual Go 1.22 compiler and `GOTOOLCHAIN=local`. Generated applications must not import the generator at runtime.

See [constructor context and I/O](../../usage/recipes/) and [generated errors](../generated-errors/) for related capabilities.
