package parser

import (
	"strings"
	"testing"

	"github.com/justyntemme/organelle/lexer"
)

func BenchmarkParseSimpleDocument(b *testing.B) {
	input := `#+TITLE: Test Document

* Headline 1
Some text here.

** Headline 2
More text.

* Headline 3
Final text.
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		p := New(l)
		_ = p.ParseDocument()
	}
}

func BenchmarkParseComplexDocument(b *testing.B) {
	input := `#+TITLE: Complex Document
#+AUTHOR: Test Author
#+DATE: 2024-01-15

* TODO [#A] Project Alpha :project:urgent:
:PROPERTIES:
:ID: proj-001
:CREATED: 2024-01-01
:END:

This is the project overview with *bold* and /italic/ text.

** DONE Research Phase
- [X] Market research
- [X] Competitor analysis
- [ ] User interviews

#+BEGIN_SRC python
def analyze():
    return data.process()
#+END_SRC

** TODO Implementation Phase

| Task | Status | Owner |
|------+--------+-------|
| Design | Done | Alice |
| Code | WIP | Bob |

Check [[https://example.com][Example]] for more info.

* References
** Documentation
** Links
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		p := New(l)
		_ = p.ParseDocument()
	}
}

func BenchmarkParseLargeDocument(b *testing.B) {
	// Generate a large document
	var builder strings.Builder
	builder.WriteString("#+TITLE: Large Document\n\n")

	for i := 0; i < 100; i++ {
		builder.WriteString("* Headline ")
		builder.WriteString(string(rune('0' + i%10)))
		builder.WriteString(" :tag1:tag2:\n")
		builder.WriteString("Some paragraph text with *bold* and /italic/ formatting.\n")
		builder.WriteString("- List item 1\n")
		builder.WriteString("- List item 2\n")
		builder.WriteString("- List item 3\n\n")
	}

	input := builder.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		p := New(l)
		_ = p.ParseDocument()
	}
}

func BenchmarkLexer(b *testing.B) {
	input := `#+TITLE: Test
* Headline 1
** Headline 2
- Item 1
- Item 2
| A | B |
|---+---|
| 1 | 2 |
`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		for {
			tok := l.NextToken()
			if tok.Type == "EOF" {
				break
			}
		}
	}
}

func BenchmarkParseInlineFormatting(b *testing.B) {
	input := `This is a paragraph with *bold*, /italic/, ~code~, =verbatim=, +strikethrough+, and _underline_ text. Also a [[https://example.com][link here]].`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l := lexer.New(input)
		p := New(l)
		_ = p.ParseDocument()
	}
}
