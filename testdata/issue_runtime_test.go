package main

import "testing"

func TestRuntimeRequirements(t *testing.T) {
	root, err := NewRoot("app", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	parent := root.NewParent()
	child := parent.NewParentChild()
	root.Commands["parent"] = func() Cmd { return parent }
	parent.SubCommands["child"] = func() Cmd { return child }

	var events []string
	root.CommandAction = func(*RootCmd) error {
		events = append(events, "root")
		return nil
	}
	child.CommandAction = func(*ParentChild) error {
		events = append(events, "child")
		return nil
	}

	err = root.Execute([]string{
		"--config", "config.yml", "parent", "--dir", "workspace", "child",
		"-zxwq=123", "-Vfirst", "-V=second", "--ptr=0",
		"--parsed", "value", "--local-parsed", "value",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := child.dir, "workspace"; got != want {
		t.Fatalf("inherited parent value = %q, want %q", got, want)
	}
	if !child.z || !child.x || !child.w || child.q != "123" {
		t.Fatalf("GNU short flags not parsed: z=%t x=%t w=%t q=%q", child.z, child.x, child.w, child.q)
	}
	if len(child.values) != 2 || child.values[0] != "first" || child.values[1] != "second" {
		t.Fatalf("repeatable values = %#v", child.values)
	}
	if child.ptr == nil || *child.ptr != 0 {
		t.Fatalf("pointer value = %v, want non-nil pointer to zero", child.ptr)
	}
	if child.parsed != "imported:value" || child.localParsed != "local:value" || child.generated != "generated" {
		t.Fatalf("parser/generator values = %q, %q, %q", child.parsed, child.localParsed, child.generated)
	}
	if len(events) != 2 || events[0] != "root" || events[1] != "child" {
		t.Fatalf("action order = %#v", events)
	}
	second := parent.NewParentChild()
	second.CommandAction = func(*ParentChild) error { return nil }
	if err := second.Execute([]string{"-v==123"}); err != nil {
		t.Fatal(err)
	}
	if second.value != "=123" {
		t.Fatalf("-v==123 parsed value = %q, want =123", second.value)
	}

	missing, err := NewRoot("app", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := missing.Execute(nil); err == nil {
		t.Fatal("missing required root flag did not fail")
	}
}
