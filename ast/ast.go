package ast

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/justyntemme/organelle/token"
)

// Node is the base interface for all AST nodes
type Node interface {
	TokenLiteral() string
	String() string
}

// Statement represents a block-level element (Headline, Paragraph)
type Statement interface {
	Node
	statementNode()
}

// Document is the root node of the AST
type Document struct {
	Children []Node
}

func (d *Document) TokenLiteral() string {
	if len(d.Children) > 0 {
		return d.Children[0].TokenLiteral()
	}
	return ""
}

func (d *Document) String() string {
	var out bytes.Buffer
	for _, s := range d.Children {
		out.WriteString(s.String())
	}
	return out.String()
}

// Headline represents a generic Org headline (* Title)
// It is recursive; it can contain other Nodes (nested headlines or paragraphs)
type Headline struct {
	Token    token.Token // The '*' token
	Level    int
	Keyword  string   // TODO, DONE, or empty
	Priority string   // A, B, C or empty
	Title    string
	Tags     []string // :tag1:tag2: parsed as ["tag1", "tag2"]
	Children []Node
}

func (h *Headline) statementNode()       {}
func (h *Headline) TokenLiteral() string { return h.Token.Literal }
func (h *Headline) String() string {
	var out bytes.Buffer
	out.WriteString(strings.Repeat("*", h.Level))
	out.WriteString(" ")
	if h.Keyword != "" {
		out.WriteString(h.Keyword)
		out.WriteString(" ")
	}
	if h.Priority != "" {
		out.WriteString("[#")
		out.WriteString(h.Priority)
		out.WriteString("] ")
	}
	out.WriteString(h.Title)
	if len(h.Tags) > 0 {
		out.WriteString(" :")
		out.WriteString(strings.Join(h.Tags, ":"))
		out.WriteString(":")
	}
	out.WriteString("\n")
	for _, c := range h.Children {
		out.WriteString(c.String())
	}
	return out.String()
}

// Paragraph represents a block of text (may contain inline elements)
type Paragraph struct {
	Token   token.Token
	Content string
	Inline  []InlineElement // Parsed inline elements (bold, italic, links, etc.)
}

func (p *Paragraph) statementNode()       {}
func (p *Paragraph) TokenLiteral() string { return p.Token.Literal }
func (p *Paragraph) String() string {
	return p.Content + "\n"
}

// InlineElement represents inline formatting within text
// It supports nesting via the Children field
type InlineElement struct {
	Type     InlineType
	Content  string          // Raw content (for text, code, verbatim - non-nestable types)
	URL      string          // For links
	Children []InlineElement // Nested inline elements (for bold, italic, etc.)
}

type InlineType int

const (
	InlineText InlineType = iota
	InlineBold
	InlineItalic
	InlineCode
	InlineVerbatim
	InlineStrikethrough
	InlineUnderline
	InlineLink
)

// String returns the string representation of an InlineType
func (t InlineType) String() string {
	switch t {
	case InlineText:
		return "text"
	case InlineBold:
		return "bold"
	case InlineItalic:
		return "italic"
	case InlineCode:
		return "code"
	case InlineVerbatim:
		return "verbatim"
	case InlineStrikethrough:
		return "strikethrough"
	case InlineUnderline:
		return "underline"
	case InlineLink:
		return "link"
	default:
		return "unknown"
	}
}

// PlainText extracts plain text content from an InlineElement, recursively
func (e *InlineElement) PlainText() string {
	if e.Type == InlineText || e.Type == InlineCode || e.Type == InlineVerbatim {
		return e.Content
	}
	var result strings.Builder
	for _, child := range e.Children {
		result.WriteString(child.PlainText())
	}
	return result.String()
}

// Keyword represents buffer settings like #+TITLE:
type Keyword struct {
	Token token.Token
	Key   string
	Value string
}

func (k *Keyword) statementNode()       {}
func (k *Keyword) TokenLiteral() string { return k.Token.Literal }
func (k *Keyword) String() string {
	return fmt.Sprintf("#+%s: %s\n", k.Key, k.Value)
}

// Block represents #+BEGIN_X ... #+END_X blocks
type Block struct {
	Token    token.Token
	Type     string // SRC, QUOTE, EXAMPLE, VERSE, CENTER, EXPORT, etc.
	Language string // For SRC blocks: python, go, etc.
	Params   string // Additional parameters after language
	Content  string
}

func (b *Block) statementNode()       {}
func (b *Block) TokenLiteral() string { return b.Token.Literal }
func (b *Block) String() string {
	var out bytes.Buffer
	out.WriteString("#+BEGIN_")
	out.WriteString(b.Type)
	if b.Language != "" {
		out.WriteString(" ")
		out.WriteString(b.Language)
	}
	if b.Params != "" {
		out.WriteString(" ")
		out.WriteString(b.Params)
	}
	out.WriteString("\n")
	out.WriteString(b.Content)
	if !strings.HasSuffix(b.Content, "\n") && b.Content != "" {
		out.WriteString("\n")
	}
	out.WriteString("#+END_")
	out.WriteString(b.Type)
	out.WriteString("\n")
	return out.String()
}

// Drawer represents :DRAWERNAME: ... :END: blocks
type Drawer struct {
	Token      token.Token
	Name       string
	Properties map[string]string // For PROPERTIES drawer
	Content    string            // Raw content for other drawers
}

func (d *Drawer) statementNode()       {}
func (d *Drawer) TokenLiteral() string { return d.Token.Literal }
func (d *Drawer) String() string {
	var out bytes.Buffer
	out.WriteString(":")
	out.WriteString(d.Name)
	out.WriteString(":\n")
	if d.Name == "PROPERTIES" {
		for k, v := range d.Properties {
			out.WriteString(":")
			out.WriteString(k)
			out.WriteString(": ")
			out.WriteString(v)
			out.WriteString("\n")
		}
	} else {
		out.WriteString(d.Content)
	}
	out.WriteString(":END:\n")
	return out.String()
}

// List represents ordered or unordered lists
type List struct {
	Token   token.Token
	Ordered bool
	Items   []*ListItem
}

func (l *List) statementNode()       {}
func (l *List) TokenLiteral() string { return l.Token.Literal }
func (l *List) String() string {
	var out bytes.Buffer
	for i, item := range l.Items {
		if l.Ordered {
			out.WriteString(fmt.Sprintf("%d. ", i+1))
		} else {
			out.WriteString("- ")
		}
		out.WriteString(item.String())
	}
	return out.String()
}

// ListItem represents a single item in a list
type ListItem struct {
	Token       token.Token
	Indent      int           // Indentation level (number of spaces/tabs)
	Checkbox    CheckboxState
	Content     string
	Children    []Node // Nested content (paragraphs, sub-lists)
}

type CheckboxState int

const (
	CheckboxNone CheckboxState = iota
	CheckboxUnchecked // [ ]
	CheckboxChecked   // [X]
	CheckboxPartial   // [-]
)

func (li *ListItem) statementNode()       {}
func (li *ListItem) TokenLiteral() string { return li.Token.Literal }
func (li *ListItem) String() string {
	var out bytes.Buffer
	switch li.Checkbox {
	case CheckboxUnchecked:
		out.WriteString("[ ] ")
	case CheckboxChecked:
		out.WriteString("[X] ")
	case CheckboxPartial:
		out.WriteString("[-] ")
	}
	out.WriteString(li.Content)
	out.WriteString("\n")
	for _, c := range li.Children {
		out.WriteString("  ")
		out.WriteString(c.String())
	}
	return out.String()
}

// Table represents org-mode tables
type Table struct {
	Token token.Token
	Rows  []*TableRow
}

func (t *Table) statementNode()       {}
func (t *Table) TokenLiteral() string { return t.Token.Literal }
func (t *Table) String() string {
	var out bytes.Buffer
	for _, row := range t.Rows {
		out.WriteString(row.String())
	}
	return out.String()
}

// TableRow represents a single row in a table
type TableRow struct {
	Token     token.Token
	Cells     []string
	Separator bool // true if this is a |---+---| separator row
}

func (tr *TableRow) statementNode()       {}
func (tr *TableRow) TokenLiteral() string { return tr.Token.Literal }
func (tr *TableRow) String() string {
	if tr.Separator {
		return "|" + strings.Repeat("-", 10) + "|\n"
	}
	return "| " + strings.Join(tr.Cells, " | ") + " |\n"
}

// Timestamp represents org-mode timestamps
type Timestamp struct {
	Token    token.Token
	Active   bool   // <...> is active, [...] is inactive
	Date     string // 2024-01-01
	Time     string // 10:00 (optional)
	Repeat   string // +1w, .+1d, ++1m (optional)
	Warning  string // -3d (optional)
	EndDate  string // For ranges: <2024-01-01>--<2024-01-02>
	EndTime  string
}

func (ts *Timestamp) statementNode()       {}
func (ts *Timestamp) TokenLiteral() string { return ts.Token.Literal }
func (ts *Timestamp) String() string {
	var out bytes.Buffer
	if ts.Active {
		out.WriteString("<")
	} else {
		out.WriteString("[")
	}
	out.WriteString(ts.Date)
	if ts.Time != "" {
		out.WriteString(" ")
		out.WriteString(ts.Time)
	}
	if ts.Repeat != "" {
		out.WriteString(" ")
		out.WriteString(ts.Repeat)
	}
	if ts.Warning != "" {
		out.WriteString(" ")
		out.WriteString(ts.Warning)
	}
	if ts.Active {
		out.WriteString(">")
	} else {
		out.WriteString("]")
	}
	if ts.EndDate != "" {
		out.WriteString("--")
		if ts.Active {
			out.WriteString("<")
		} else {
			out.WriteString("[")
		}
		out.WriteString(ts.EndDate)
		if ts.EndTime != "" {
			out.WriteString(" ")
			out.WriteString(ts.EndTime)
		}
		if ts.Active {
			out.WriteString(">")
		} else {
			out.WriteString("]")
		}
	}
	return out.String()
}

// Link represents [[url][description]] or [[url]] links
type Link struct {
	Token       token.Token
	URL         string
	Description string
}

func (l *Link) statementNode()       {}
func (l *Link) TokenLiteral() string { return l.Token.Literal }
func (l *Link) String() string {
	if l.Description != "" {
		return fmt.Sprintf("[[%s][%s]]", l.URL, l.Description)
	}
	return fmt.Sprintf("[[%s]]", l.URL)
}

// Comment represents # comment lines
type Comment struct {
	Token   token.Token
	Content string
}

func (c *Comment) statementNode()       {}
func (c *Comment) TokenLiteral() string { return c.Token.Literal }
func (c *Comment) String() string {
	return "# " + c.Content + "\n"
}

// HorizontalRule represents ----- separator lines (5+ dashes)
type HorizontalRule struct {
	Token token.Token
}

func (hr *HorizontalRule) statementNode()       {}
func (hr *HorizontalRule) TokenLiteral() string { return hr.Token.Literal }
func (hr *HorizontalRule) String() string {
	return "-----\n"
}
