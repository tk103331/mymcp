package app

import (
	"reflect"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	cases := []struct {
		input string
		want  []string
	}{
		{``, nil},
		{`-y github-mcp`, []string{"-y", "github-mcp"}},
		{`--config "/Users/me/My Config.json"`, []string{"--config", "/Users/me/My Config.json"}},
		{`--label 'hello world'`, []string{"--label", "hello world"}},
		{`--json "{\"key\": \"value with spaces\"}"`, []string{"--json", `{"key": "value with spaces"}`}},
		{`a\ b c`, []string{"a b", "c"}},
		{`"a\nb"`, []string{"a\nb"}},
	}
	for _, tc := range cases {
		got, err := splitArgs(tc.input)
		if err != nil {
			t.Fatalf("splitArgs(%q): %v", tc.input, err)
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitArgs(%q) = %#v, want %#v", tc.input, got, tc.want)
		}
	}
	if _, err := splitArgs(`"unterminated`); err == nil {
		t.Fatal("unterminated quote was accepted")
	}
}

func TestJoinArgsRoundTrip(t *testing.T) {
	args := []string{"-y", "github-mcp", "--config", "/Users/me/My Config.json", "", `a "quoted" value`}
	encoded := joinArgs(args)
	decoded, err := splitArgs(encoded)
	if err != nil {
		t.Fatalf("joinArgs(%q): %v", encoded, err)
	}
	if !reflect.DeepEqual(decoded, args) {
		t.Fatalf("round trip = %#v, want %#v (encoded %q)", decoded, args, encoded)
	}
}
