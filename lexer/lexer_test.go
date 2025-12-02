package lexer

import (
	"testing"

	"github.com/justyntemme/organelle/token"
)

func TestNextToken(t *testing.T) {
	input := `* Headline 1
** Headline 2
#+TITLE: My File
Some paragraph text.`

	tests := []struct {
		expectedType    token.TokenType
		expectedLiteral string
	}{
		{token.STARS, "*"},
		{token.TEXT, " Headline 1"}, // Simplifying: Spaces included in text for now
		{token.NEWLINE, "\n"},
		{token.STARS, "**"},
		{token.TEXT, " Headline 2"},
		{token.NEWLINE, "\n"},
		{token.KEYWORD, "#+TITLE: My File"},
		{token.NEWLINE, "\n"},
		{token.TEXT, "Some paragraph text."},
		{token.EOF, ""},
	}

	l := New(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q",
				i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q",
				i, tt.expectedLiteral, tok.Literal)
		}
	}
}
