package parser

import (
	"testing"
)

func TestIssue55_SanitizeToIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", "Cmd"},
		{"StartsWithDigit", "123test", "Cmd123test"},
		{"StartsWithDigitHyphen", "123-test", "Cmd123Test"},
		{"OnlyDigits", "123", "Cmd123"},
		{"StartsWithInvalid", "@foo", "Foo"},
		{"StartsWithInvalidSpace", " foo", "Foo"},
		{"StartsWithInvalidHyphen", "-foo", "Foo"},
		{"StartsWithInvalidThenDigit", "_123", "Cmd123"},
		{"MixedInvalid", "foo@bar", "FooBar"},
		{"MixedInvalid2", "foo#bar", "FooBar"},
		{"MixedInvalid3", "foo$bar", "FooBar"},
		{"MixedInvalidNumbers", "foo123bar", "Foo123bar"},
		{"MixedInvalidNumbers2", "foo-123-bar", "Foo123Bar"},
		{"Complex", "123-foo@bar.com", "Cmd123FooBarCom"},
		{"JustInvalid", "!@#$", "Cmd"},
		{"Unicode", "héllo", "Héllo"},
		{"UnicodeStart", "éllo", "Éllo"},
		{"UnicodeDigit", "1éllo", "Cmd1éllo"},
		{"UnicodeFullWidthDigit", "１test", "Cmd１test"}, // Full-width digit One
		{"Emoji", "a💩b", "AB"},
		{"JustEmoji", "💩", "Cmd"},
		{"EmojiStart", "💩a", "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeToIdentifier(tt.input); got != tt.expected {
				t.Errorf("SanitizeToIdentifier(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIssue55_ToKebabCase(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Simple", "Test", "test"},
		{"NumberPrefix", "123Test", "123-test"},
		{"NumberPrefix2", "123test", "123test"},
		{"CmdPrefix", "Cmd123Test", "cmd123-test"},
		{"CmdPrefix2", "Cmd123test", "cmd123test"},
		{"Acronym", "JSONData", "json-data"},
		{"AcronymNumber", "JSON123Data", "json123-data"},
		{"AcronymNumber2", "JSON123data", "json123data"},
		{"NumberInMiddle", "My123Test", "my123-test"},
		{"NumberInMiddle2", "My123test", "my123test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToKebabCase(tt.input); got != tt.expected {
				t.Errorf("ToKebabCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
