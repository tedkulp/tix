package utils

import (
	"reflect"
	"testing"
	"time"
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

func TestGenerateMilestone(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{
			name: "Q1",
			date: time.Date(2025, time.January, 15, 0, 0, 0, 0, time.UTC),
			want: "2025.Q1",
		},
		{
			name: "Q1 edge - March",
			date: time.Date(2025, time.March, 31, 23, 59, 59, 0, time.UTC),
			want: "2025.Q1",
		},
		{
			name: "Q2",
			date: time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC),
			want: "2025.Q2",
		},
		{
			name: "Q3",
			date: time.Date(2025, time.July, 15, 0, 0, 0, 0, time.UTC),
			want: "2025.Q3",
		},
		{
			name: "Q4",
			date: time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			want: "2025.Q4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMilestone(tt.date)
			if got != tt.want {
				t.Errorf("GenerateMilestone() = %v, want %v", got, tt.want)
			}
		})
	}
}
