package ansi

import (
	"strings"
	"testing"
)

func TestAnsiParser_ResetOnNewline(t *testing.T) {
	var runes strings.Builder
	var escapes []EscapeSequence

	parser := &AnsiParser{
		Rune: func(r rune) {
			runes.WriteRune(r)
		},
		Escape: func(e EscapeSequence) {
			escapes = append(escapes, e)
		},
	}

	// Broken CSI sequence cut off by newline: \033[1;31\n
	input := []byte("Line 1 \033[1;31\nLine 2 text\n")
	if err := parser.ConvertFromUTF8(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := runes.String()
	want := "Line 1 \nLine 2 text\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnsiParser_ResetOnNewlineInEscaping(t *testing.T) {
	var runes strings.Builder

	parser := &AnsiParser{
		Rune: func(r rune) {
			runes.WriteRune(r)
		},
		Escape: func(e EscapeSequence) {},
	}

	// Broken ESC cut off by newline: \033\n
	input := []byte("Line 1 \033\nLine 2 text\n")
	if err := parser.ConvertFromUTF8(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := runes.String()
	want := "Line 1 \nLine 2 text\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAnsiParser_NormalEscape(t *testing.T) {
	var runes strings.Builder
	var modes []rune

	parser := &AnsiParser{
		Rune: func(r rune) {
			runes.WriteRune(r)
		},
		Escape: func(e EscapeSequence) {
			modes = append(modes, e.Mode)
		},
	}

	input := []byte("Line 1 \033[1;31mColored\033[m\nLine 2\n")
	if err := parser.ConvertFromUTF8(input); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := runes.String()
	want := "Line 1 Colored\nLine 2\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if len(modes) != 2 || modes[0] != 'm' || modes[1] != 'm' {
		t.Errorf("got modes %v, want ['m', 'm']", modes)
	}
}
