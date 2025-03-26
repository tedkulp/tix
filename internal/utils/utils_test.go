package utils

import (
	"reflect"
	"testing"
)

func TestTruncateAndDashCase(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "simple camelCase",
			input:  "helloWorld",
			maxLen: 20,
			want:   "hello-world",
		},
		{
			name:   "with spaces",
			input:  "hello world",
			maxLen: 20,
			want:   "hello-world",
		},
		{
			name:   "with special chars",
			input:  "hello, world!",
			maxLen: 20,
			want:   "hello-world",
		},
		{
			name:   "with apostrophe",
			input:  "user's name",
			maxLen: 20,
			want:   "user-s-name",
		},
		{
			name:   "truncation",
			input:  "thisIsAReallyLongStringThatShouldBeTruncated",
			maxLen: 11,
			want:   "this-is-a-r",
		},
		{
			name:   "consecutive special chars",
			input:  "hello  world",
			maxLen: 20,
			want:   "hello-world",
		},
		{
			name:   "starting with special char",
			input:  "!hello",
			maxLen: 20,
			want:   "hello",
		},
		{
			name:   "ending with special char",
			input:  "hello!",
			maxLen: 20,
			want:   "hello",
		},
		{
			name:   "mixed PascalCase with special chars",
			input:  "UserProfile-Name",
			maxLen: 20,
			want:   "user-profile-name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateAndDashCase(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("TruncateAndDashCase() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitOnCommaAndWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple comma separated",
			input: "a,b,c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with whitespace",
			input: "a, b, c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "mixed spacing",
			input: "a,b ,  c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "empty parts",
			input: "a,,c",
			want:  []string{"a", "", "c"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{""},
		},
		{
			name:  "only whitespace",
			input: "  ,  ,  ",
			want:  []string{"", "", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitOnCommaAndWhitespace(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitOnCommaAndWhitespace() = %v, want %v", got, tt.want)
			}
		})
	}
}
