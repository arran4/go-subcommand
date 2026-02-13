package common

import (
	"testing"
)

func TestSanitizeToIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", "Cmd"},
		{"Simple", "test", "Test"},
		{"KebabCase", "hello-world", "HelloWorld"},
		{"SnakeCase", "hello_world", "HelloWorld"},
		{"SpaceSeparated", "hello world", "HelloWorld"},
		{"MixedDelimiters", "hello-world_test", "HelloWorldTest"},
		{"StartsWithDigit", "123test", "Cmd123test"},
		{"StartsWithDigitHyphen", "123-test", "Cmd123Test"},
		{"OnlyDigits", "123", "Cmd123"},
		{"OnlyDelimiters", "---", "Cmd"},
		{"TrailingDelimiter", "test-", "Test"},
		{"LeadingDelimiter", "-test", "Test"},
		{"Unicode", "héllo-wörld", "HélloWörld"},
		{"MultipleDelimiters", "a--b", "AB"},
		{"DotSeparated", "foo.bar", "FooBar"},
		{"Complex", "foo.bar-baz_qux", "FooBarBazQux"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeToIdentifier(tt.input); got != tt.expected {
				t.Errorf("SanitizeToIdentifier(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestToKebabCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"JSONData", "json-data"},
		{"MyJSONData", "my-json-data"},
		{"HTTPServer", "http-server"},
		{"SimpleTest", "simple-test"},
		{"camelCase", "camel-case"},
		{"UserID", "user-id"},
		{"GetURLForThing", "get-url-for-thing"},
		{"Simple", "simple"},
		{"ALLCAPS", "allcaps"},
		{"StartWithDigit123", "start-with-digit123"},
		{"123StartWithDigit", "123-start-with-digit"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ToKebabCase(tt.input); got != tt.want {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNameAllocator(t *testing.T) {
	t.Run("ReserveNames", func(t *testing.T) {
		na := NewNameAllocator()
		reserved := []string{
			"Cmd", "RootCmd", "UserError",
			"NewRoot", "NewUserError", "executeUsage",
			"main", "init",
		}
		for _, r := range reserved {
			if !na.used[r] {
				t.Errorf("Expected %q to be reserved", r)
			}
		}
	})

	t.Run("AllocateUnique", func(t *testing.T) {
		na := NewNameAllocator()
		name := na.Allocate("my-func")
		if name != "MyFunc" {
			t.Errorf("Expected 'MyFunc', got %q", name)
		}
		if !na.used["MyFunc"] {
			t.Errorf("Expected 'MyFunc' to be marked as used")
		}
	})

	t.Run("AllocateCollision", func(t *testing.T) {
		na := NewNameAllocator()
		// Allocate "Cmd" which is reserved
		name := na.Allocate("cmd")
		if name != "Cmd2" {
			t.Errorf("Expected 'Cmd2', got %q", name)
		}

		// Allocate "Cmd" again
		name2 := na.Allocate("cmd")
		if name2 != "Cmd3" {
			t.Errorf("Expected 'Cmd3', got %q", name2)
		}
	})

	t.Run("AllocateCollisionWithSanitization", func(t *testing.T) {
		na := NewNameAllocator()
		na.Allocate("test") // Allocates "Test"

		name := na.Allocate("test")
		if name != "Test2" {
			t.Errorf("Expected 'Test2', got %q", name)
		}

		name2 := na.Allocate("test")
		if name2 != "Test3" {
			t.Errorf("Expected 'Test3', got %q", name2)
		}
	})

	t.Run("AllocateGeneratedNames", func(t *testing.T) {
		na := NewNameAllocator()
		// "Test" is allocated
		na.Allocate("test")
		// Force mark "Test2" as used manually (simulating another path)
		na.used["Test2"] = true

		// Now allocating "test" again should skip Test2 and go to Test3
		name := na.Allocate("test")
		if name != "Test3" {
			t.Errorf("Expected 'Test3', got %q", name)
		}
	})
}
