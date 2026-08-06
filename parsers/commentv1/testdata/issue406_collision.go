package testdata

// AppMain is a command `app`
// description: App
func AppMain() {}

// Foo1 is a subcommand `app foo`
// description: sub
func foo1() {}

// Foo2 is a subcommand `app bar foo`
// description: sub
func foo2() {}

// Foo3 is a subcommand `app baz foo`
// description: sub
func foo3() {}

// Foo4 is a subcommand `app deep nested foo`
// description: sub
func foo4() {}

// FooBar1 is a subcommand `app foo-bar`
// description: sub
func foo_bar_1() {}

// FooBar2 is a subcommand `app foo_bar`
// description: sub
func foo_bar_2() {}

// FooBar3 is a subcommand `app FooBar`
// description: sub
func foo_bar_3() {}

// NewUserError is a subcommand `app user error`
// description: sub
func new_user_error() {}

// NewLazyCommand is a subcommand `app lazy command`
// description: sub
func new_lazy_command() {}
