package testdata

import "fmt"

// AppMain is a command `app`
func AppMain() {}

// Foo1 is a subcommand `app foo`
func foo1() { fmt.Println("foo") }

// Foo2 is a subcommand `app bar foo`
func foo2() { fmt.Println("bar foo") }

// Foo3 is a subcommand `app baz foo`
func foo3() { fmt.Println("baz foo") }

// Foo4 is a subcommand `app deep nested foo`
func foo4() { fmt.Println("deep nested foo") }

// FooBar1 is a subcommand `app foo-bar`
func foo_bar_1() { fmt.Println("foo-bar") }

// FooBar2 is a subcommand `app foo_bar`
func foo_bar_2() { fmt.Println("foo_bar") }

// FooBar3 is a subcommand `app FooBar`
func foo_bar_3() { fmt.Println("FooBar") }

// NewUserError is a subcommand `app user error`
func new_user_error() { fmt.Println("user error") }

// NewLazyCommand is a subcommand `app lazy command`
func new_lazy_command() { fmt.Println("lazy command") }
