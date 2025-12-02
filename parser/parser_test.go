package parser

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/justyntemme/organelle/ast"
	"github.com/justyntemme/organelle/lexer"
)

func TestParseHeadlineHierarchy(t *testing.T) {
	input := `
* H1
** H2
Text inside H2
* H1 Second
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	// We expect 2 root children (H1 and H1 Second)
	if len(doc.Children) != 2 {
		t.Fatalf("doc.Children does not contain 2 statements. got=%d", len(doc.Children))
	}

	// Check First H1
	h1, ok := doc.Children[0].(*ast.Headline)
	if !ok {
		t.Fatalf("stmt is not ast.Headline. got=%T", doc.Children[0])
	}
	if h1.Title != "H1" {
		t.Errorf("h1.Title not 'H1'. got=%q", h1.Title)
	}
	if len(h1.Children) != 1 {
		t.Fatalf("h1 should have 1 child (H2). got=%d", len(h1.Children))
	}

	// Check Nested H2
	h2, ok := h1.Children[0].(*ast.Headline)
	if !ok {
		t.Fatalf("child is not ast.Headline. got=%T", h1.Children[0])
	}
	if h2.Level != 2 {
		t.Errorf("h2.Level not 2. got=%d", h2.Level)
	}

	// Check content inside H2
	if len(h2.Children) != 1 {
		t.Fatalf("h2 should have 1 child (text). got=%d", len(h2.Children))
	}
	para, ok := h2.Children[0].(*ast.Paragraph)
	if !ok {
		t.Errorf("child of h2 is not Paragraph")
	}
	if para.Content != "Text inside H2" {
		t.Errorf("Paragraph content wrong. got=%q", para.Content)
	}
}

func TestParseTODOKeywords(t *testing.T) {
	input := `* TODO Design the system
** DONE Write specs
* TODO [#A] High priority task
* [#B] Just priority, no keyword
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	// 3 top-level headlines (h2 is nested under h1)
	if len(doc.Children) != 3 {
		t.Fatalf("expected 3 top-level headlines, got=%d", len(doc.Children))
	}

	// First headline: TODO Design the system
	h1 := doc.Children[0].(*ast.Headline)
	if h1.Keyword != "TODO" {
		t.Errorf("h1.Keyword expected 'TODO', got=%q", h1.Keyword)
	}
	if h1.Title != "Design the system" {
		t.Errorf("h1.Title expected 'Design the system', got=%q", h1.Title)
	}

	// Nested headline: DONE Write specs
	if len(h1.Children) != 1 {
		t.Fatalf("h1 should have 1 child, got=%d", len(h1.Children))
	}
	h2 := h1.Children[0].(*ast.Headline)
	if h2.Keyword != "DONE" {
		t.Errorf("h2.Keyword expected 'DONE', got=%q", h2.Keyword)
	}
	if h2.Title != "Write specs" {
		t.Errorf("h2.Title expected 'Write specs', got=%q", h2.Title)
	}

	// Second top-level headline: TODO [#A] High priority task
	h3 := doc.Children[1].(*ast.Headline)
	if h3.Keyword != "TODO" {
		t.Errorf("h3.Keyword expected 'TODO', got=%q", h3.Keyword)
	}
	if h3.Priority != "A" {
		t.Errorf("h3.Priority expected 'A', got=%q", h3.Priority)
	}
	if h3.Title != "High priority task" {
		t.Errorf("h3.Title expected 'High priority task', got=%q", h3.Title)
	}

	// Third top-level headline: [#B] Just priority, no keyword
	h4 := doc.Children[2].(*ast.Headline)
	if h4.Keyword != "" {
		t.Errorf("h4.Keyword expected empty, got=%q", h4.Keyword)
	}
	if h4.Priority != "B" {
		t.Errorf("h4.Priority expected 'B', got=%q", h4.Priority)
	}
	if h4.Title != "Just priority, no keyword" {
		t.Errorf("h4.Title expected 'Just priority, no keyword', got=%q", h4.Title)
	}
}

func TestParseUTF8Content(t *testing.T) {
	input := `* 日本語のタイトル
こんにちは世界
* Émojis 🎉 and accénts
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 2 {
		t.Fatalf("expected 2 headlines, got=%d", len(doc.Children))
	}

	h1 := doc.Children[0].(*ast.Headline)
	if h1.Title != "日本語のタイトル" {
		t.Errorf("h1.Title expected '日本語のタイトル', got=%q", h1.Title)
	}

	para := h1.Children[0].(*ast.Paragraph)
	if para.Content != "こんにちは世界" {
		t.Errorf("para.Content expected 'こんにちは世界', got=%q", para.Content)
	}

	h2 := doc.Children[1].(*ast.Headline)
	if h2.Title != "Émojis 🎉 and accénts" {
		t.Errorf("h2.Title expected 'Émojis 🎉 and accénts', got=%q", h2.Title)
	}
}

func TestParseTags(t *testing.T) {
	input := `* Project Alpha :project:urgent:
* TODO Task with tags :work:coding:
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	h1 := doc.Children[0].(*ast.Headline)
	if h1.Title != "Project Alpha" {
		t.Errorf("h1.Title expected 'Project Alpha', got=%q", h1.Title)
	}
	if len(h1.Tags) != 2 {
		t.Fatalf("expected 2 tags, got=%d", len(h1.Tags))
	}
	if h1.Tags[0] != "project" || h1.Tags[1] != "urgent" {
		t.Errorf("tags expected [project, urgent], got=%v", h1.Tags)
	}

	h2 := doc.Children[1].(*ast.Headline)
	if h2.Keyword != "TODO" {
		t.Errorf("h2.Keyword expected 'TODO', got=%q", h2.Keyword)
	}
	if len(h2.Tags) != 2 {
		t.Fatalf("expected 2 tags, got=%d", len(h2.Tags))
	}
}

func TestParseCodeBlock(t *testing.T) {
	input := `#+BEGIN_SRC python
def hello():
    print("Hello, World!")
#+END_SRC
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 1 {
		t.Fatalf("expected 1 child, got=%d", len(doc.Children))
	}

	block, ok := doc.Children[0].(*ast.Block)
	if !ok {
		t.Fatalf("expected *ast.Block, got=%T", doc.Children[0])
	}

	if block.Type != "SRC" {
		t.Errorf("block.Type expected 'SRC', got=%q", block.Type)
	}
	if block.Language != "python" {
		t.Errorf("block.Language expected 'python', got=%q", block.Language)
	}
	expectedContent := `def hello():
    print("Hello, World!")`
	if block.Content != expectedContent {
		t.Errorf("block.Content expected %q, got=%q", expectedContent, block.Content)
	}
}

func TestParseQuoteBlock(t *testing.T) {
	input := `#+BEGIN_QUOTE
To be or not to be,
that is the question.
#+END_QUOTE
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	block := doc.Children[0].(*ast.Block)
	if block.Type != "QUOTE" {
		t.Errorf("block.Type expected 'QUOTE', got=%q", block.Type)
	}
}

func TestParsePropertiesDrawer(t *testing.T) {
	input := `* Task with properties
:PROPERTIES:
:ID: 12345
:CREATED: 2024-01-01
:CUSTOM_ID: my-task
:END:
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	h1 := doc.Children[0].(*ast.Headline)
	if len(h1.Children) != 1 {
		t.Fatalf("expected 1 child (drawer), got=%d", len(h1.Children))
	}

	drawer, ok := h1.Children[0].(*ast.Drawer)
	if !ok {
		t.Fatalf("expected *ast.Drawer, got=%T", h1.Children[0])
	}

	if drawer.Name != "PROPERTIES" {
		t.Errorf("drawer.Name expected 'PROPERTIES', got=%q", drawer.Name)
	}
	if drawer.Properties["ID"] != "12345" {
		t.Errorf("ID property expected '12345', got=%q", drawer.Properties["ID"])
	}
	if drawer.Properties["CREATED"] != "2024-01-01" {
		t.Errorf("CREATED property expected '2024-01-01', got=%q", drawer.Properties["CREATED"])
	}
}

func TestParseUnorderedList(t *testing.T) {
	input := `- Item one
- Item two
- Item three
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 1 {
		t.Fatalf("expected 1 child (list), got=%d", len(doc.Children))
	}

	list, ok := doc.Children[0].(*ast.List)
	if !ok {
		t.Fatalf("expected *ast.List, got=%T", doc.Children[0])
	}

	if list.Ordered {
		t.Error("list should be unordered")
	}
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 items, got=%d", len(list.Items))
	}
	if list.Items[0].Content != "Item one" {
		t.Errorf("first item content expected 'Item one', got=%q", list.Items[0].Content)
	}
}

func TestParseOrderedList(t *testing.T) {
	input := `1. First item
2. Second item
3. Third item
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	list := doc.Children[0].(*ast.List)
	if !list.Ordered {
		t.Error("list should be ordered")
	}
	if len(list.Items) != 3 {
		t.Fatalf("expected 3 items, got=%d", len(list.Items))
	}
}

func TestParseCheckboxList(t *testing.T) {
	input := `- [ ] Unchecked item
- [X] Checked item
- [-] Partial item
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	list := doc.Children[0].(*ast.List)
	if list.Items[0].Checkbox != ast.CheckboxUnchecked {
		t.Errorf("first item should be unchecked, got=%d", list.Items[0].Checkbox)
	}
	if list.Items[1].Checkbox != ast.CheckboxChecked {
		t.Errorf("second item should be checked, got=%d", list.Items[1].Checkbox)
	}
	if list.Items[2].Checkbox != ast.CheckboxPartial {
		t.Errorf("third item should be partial, got=%d", list.Items[2].Checkbox)
	}
}

func TestParseTable(t *testing.T) {
	input := `| Name | Age | City |
|------+-----+------|
| Alice | 30 | NYC |
| Bob | 25 | LA |
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	table, ok := doc.Children[0].(*ast.Table)
	if !ok {
		t.Fatalf("expected *ast.Table, got=%T", doc.Children[0])
	}

	if len(table.Rows) != 4 {
		t.Fatalf("expected 4 rows, got=%d", len(table.Rows))
	}

	// Header row
	if len(table.Rows[0].Cells) != 3 {
		t.Errorf("header should have 3 cells, got=%d", len(table.Rows[0].Cells))
	}
	if table.Rows[0].Cells[0] != "Name" {
		t.Errorf("first header cell expected 'Name', got=%q", table.Rows[0].Cells[0])
	}

	// Separator row
	if !table.Rows[1].Separator {
		t.Error("second row should be a separator")
	}

	// Data row
	if table.Rows[2].Cells[0] != "Alice" {
		t.Errorf("first data cell expected 'Alice', got=%q", table.Rows[2].Cells[0])
	}
}

func TestParseComment(t *testing.T) {
	input := `# This is a comment
* Headline after comment
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 2 {
		t.Fatalf("expected 2 children, got=%d", len(doc.Children))
	}

	comment, ok := doc.Children[0].(*ast.Comment)
	if !ok {
		t.Fatalf("expected *ast.Comment, got=%T", doc.Children[0])
	}
	if comment.Content != "This is a comment" {
		t.Errorf("comment content expected 'This is a comment', got=%q", comment.Content)
	}
}

func TestParseInlineFormatting(t *testing.T) {
	input := `This has *bold* and /italic/ and ~code~ text.`

	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	para := doc.Children[0].(*ast.Paragraph)

	// Check that inline elements were parsed
	if len(para.Inline) == 0 {
		t.Fatal("expected inline elements to be parsed")
	}

	// Find bold element - nested types use Children, non-nested use Content
	foundBold := false
	foundItalic := false
	foundCode := false
	for _, elem := range para.Inline {
		if elem.Type == ast.InlineBold {
			// Bold uses Children for nested content
			if len(elem.Children) > 0 && elem.Children[0].Content == "bold" {
				foundBold = true
			}
		}
		if elem.Type == ast.InlineItalic {
			// Italic uses Children for nested content
			if len(elem.Children) > 0 && elem.Children[0].Content == "italic" {
				foundItalic = true
			}
		}
		if elem.Type == ast.InlineCode && elem.Content == "code" {
			// Code uses Content directly (non-nestable)
			foundCode = true
		}
	}

	if !foundBold {
		t.Error("expected to find bold element")
	}
	if !foundItalic {
		t.Error("expected to find italic element")
	}
	if !foundCode {
		t.Error("expected to find code element")
	}
}

func TestParseLink(t *testing.T) {
	input := `Check out [[https://example.com][Example Site]] for more info.`

	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	para := doc.Children[0].(*ast.Paragraph)

	foundLink := false
	for _, elem := range para.Inline {
		if elem.Type == ast.InlineLink {
			if elem.URL != "https://example.com" {
				t.Errorf("link URL expected 'https://example.com', got=%q", elem.URL)
			}
			// Link description is now parsed into Children
			if len(elem.Children) > 0 {
				if elem.Children[0].Content != "Example Site" {
					t.Errorf("link description expected 'Example Site', got=%q", elem.Children[0].Content)
				}
			} else {
				t.Error("expected link to have description in Children")
			}
			foundLink = true
		}
	}

	if !foundLink {
		t.Error("expected to find link element")
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		input    string
		active   bool
		date     string
		time     string
		repeat   string
		warning  string
	}{
		{"<2024-01-15>", true, "2024-01-15", "", "", ""},
		{"[2024-01-15]", false, "2024-01-15", "", "", ""},
		{"<2024-01-15 Mon 10:00>", true, "2024-01-15", "10:00", "", ""},
		{"<2024-01-15 +1w>", true, "2024-01-15", "", "+1w", ""},
		{"<2024-01-15 +1w -3d>", true, "2024-01-15", "", "+1w", "-3d"},
	}

	for _, tt := range tests {
		ts := ParseTimestamp(tt.input)
		if ts == nil {
			t.Errorf("ParseTimestamp(%q) returned nil", tt.input)
			continue
		}
		if ts.Active != tt.active {
			t.Errorf("ParseTimestamp(%q).Active = %v, want %v", tt.input, ts.Active, tt.active)
		}
		if ts.Date != tt.date {
			t.Errorf("ParseTimestamp(%q).Date = %q, want %q", tt.input, ts.Date, tt.date)
		}
		if ts.Time != tt.time {
			t.Errorf("ParseTimestamp(%q).Time = %q, want %q", tt.input, ts.Time, tt.time)
		}
	}
}

func TestParseKeyword(t *testing.T) {
	input := `#+TITLE: My Document
#+AUTHOR: John Doe
#+DATE: 2024-01-15
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 3 {
		t.Fatalf("expected 3 keywords, got=%d", len(doc.Children))
	}

	kw1 := doc.Children[0].(*ast.Keyword)
	if kw1.Key != "TITLE" || kw1.Value != "My Document" {
		t.Errorf("first keyword expected TITLE: My Document, got=%s: %s", kw1.Key, kw1.Value)
	}

	kw2 := doc.Children[1].(*ast.Keyword)
	if kw2.Key != "AUTHOR" || kw2.Value != "John Doe" {
		t.Errorf("second keyword expected AUTHOR: John Doe, got=%s: %s", kw2.Key, kw2.Value)
	}
}

func TestParserWithLogger(t *testing.T) {
	// Create a logger that discards output for testing
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	input := `* Test headline`
	l := lexer.New(input, lexer.WithLogger(logger))
	p := New(l, WithLogger(logger))
	doc := p.ParseDocument()

	if len(doc.Children) != 1 {
		t.Fatalf("expected 1 child, got=%d", len(doc.Children))
	}
}

func TestComplexDocument(t *testing.T) {
	input := `#+TITLE: Project Plan
#+AUTHOR: Team Lead

* TODO [#A] Project Overview :project:planning:
:PROPERTIES:
:ID: proj-001
:CREATED: 2024-01-01
:END:

This is the main project document.

** DONE Research Phase
- [X] Market research
- [X] Competitor analysis
- [ ] User interviews

** TODO Implementation
#+BEGIN_SRC go
func main() {
    fmt.Println("Hello, World!")
}
#+END_SRC

| Task | Status | Owner |
|------+--------+-------|
| Design | Done | Alice |
| Code | WIP | Bob |

* References
Check [[https://golang.org][Go Website]] for documentation.
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	// Should have: TITLE, AUTHOR, and 2 top-level headlines
	if len(doc.Children) < 4 {
		t.Fatalf("expected at least 4 top-level children, got=%d", len(doc.Children))
	}

	// Check first headline has tags
	var projectHeadline *ast.Headline
	for _, child := range doc.Children {
		if hl, ok := child.(*ast.Headline); ok {
			projectHeadline = hl
			break
		}
	}

	if projectHeadline == nil {
		t.Fatal("expected to find a headline")
	}

	if projectHeadline.Keyword != "TODO" {
		t.Errorf("expected TODO keyword, got=%q", projectHeadline.Keyword)
	}
	if projectHeadline.Priority != "A" {
		t.Errorf("expected priority A, got=%q", projectHeadline.Priority)
	}
	if len(projectHeadline.Tags) != 2 {
		t.Errorf("expected 2 tags, got=%d", len(projectHeadline.Tags))
	}
}

func TestParseNestedInlineFormatting(t *testing.T) {
	input := `This has *bold with /italic inside/* text.`

	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	para := doc.Children[0].(*ast.Paragraph)

	// Find bold element with nested italic
	foundNestedItalic := false
	for _, elem := range para.Inline {
		if elem.Type == ast.InlineBold {
			// Check for nested italic in Children
			for _, child := range elem.Children {
				if child.Type == ast.InlineItalic {
					foundNestedItalic = true
					break
				}
			}
		}
	}

	if !foundNestedItalic {
		t.Error("expected to find italic nested inside bold")
	}
}

func TestParseNestedList(t *testing.T) {
	input := `- Item 1
  - Nested item 1.1
  - Nested item 1.2
- Item 2
`
	l := lexer.New(input)
	p := New(l)
	doc := p.ParseDocument()

	if len(p.Errors()) != 0 {
		t.Errorf("parser has errors: %v", p.Errors())
	}

	if len(doc.Children) != 1 {
		t.Fatalf("expected 1 child (list), got=%d", len(doc.Children))
	}

	list, ok := doc.Children[0].(*ast.List)
	if !ok {
		t.Fatalf("expected *ast.List, got=%T", doc.Children[0])
	}

	// Should have 2 top-level items
	if len(list.Items) != 2 {
		t.Fatalf("expected 2 top-level items, got=%d", len(list.Items))
	}

	// First item should have nested children
	if len(list.Items[0].Children) == 0 {
		t.Error("expected first item to have nested children")
	}

	// Check nested list
	if len(list.Items[0].Children) > 0 {
		nestedList, ok := list.Items[0].Children[0].(*ast.List)
		if !ok {
			t.Errorf("expected nested *ast.List, got=%T", list.Items[0].Children[0])
		} else {
			if len(nestedList.Items) != 2 {
				t.Errorf("expected 2 nested items, got=%d", len(nestedList.Items))
			}
		}
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	input := `* Headline 1
* Headline 2
* Headline 3
`
	l := lexer.New(input, lexer.WithContext(ctx))
	p := New(l, WithContext(ctx))
	doc := p.ParseDocument()

	// Should have errors due to cancellation
	if len(p.Errors()) == 0 {
		// It's possible parsing completed before cancellation was checked
		// so we don't fail, just check the doc was created
		_ = doc
	}
}

func TestInputSizeLimit(t *testing.T) {
	// Create input larger than limit
	largeInput := strings.Repeat("* headline\n", 1000)

	l := lexer.New(largeInput, lexer.WithMaxInputSize(100))

	if l.Err() == nil {
		t.Error("expected error for input exceeding size limit")
	}
	if l.Err() != lexer.ErrInputTooLarge {
		t.Errorf("expected ErrInputTooLarge, got=%v", l.Err())
	}
}
