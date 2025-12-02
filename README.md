# Organelle

Organelle is a high-performance, thread-safe Go library for parsing Org-mode files into an Abstract Syntax Tree (AST). It is designed for tools that need to convert Org files to other formats (like HTML), build static site generators, or create rich text previews.

## Features

- **Fast & Lightweight**: Uses a custom state-based lexer instead of regular expressions for high throughput
- **Thread Safe**: Parsers are isolated struct instances, safe for concurrent use in goroutines
- **AST Focused**: Produces a structured tree (Document -> Headlines -> Children) rather than a flat list of tokens
- **Full Org-mode Support**: Headlines, TODO/DONE, priorities, tags, code blocks, lists, tables, drawers, timestamps, and inline formatting
- **Context Support**: Cancellable parsing with `context.Context`
- **Structured Logging**: Built-in `slog` support for debugging
- **Input Validation**: Configurable size limits to prevent resource exhaustion
- **Standard Go**: Minimal dependencies (only standard library)

## Installation

```bash
go get github.com/justyntemme/organelle
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/justyntemme/organelle/ast"
    "github.com/justyntemme/organelle/lexer"
    "github.com/justyntemme/organelle/parser"
)

func main() {
    input := `
* Project Alpha :project:
#+AUTHOR: Jane Doe
** TODO [#A] Design Phase
We need to draft the initial specifications.
- [ ] Write specs
- [X] Review existing code
`

    // 1. Create the Lexer
    l := lexer.New(input)

    // 2. Create the Parser
    p := parser.New(l)

    // 3. Parse into AST
    doc := p.ParseDocument()

    // Check for errors
    if len(p.Errors()) > 0 {
        for _, err := range p.Errors() {
            fmt.Println("Error:", err)
        }
        return
    }

    // 4. Traverse the AST
    for _, node := range doc.Children {
        printNode(node, 0)
    }
}

func printNode(node ast.Node, indent int) {
    prefix := ""
    for i := 0; i < indent; i++ {
        prefix += "  "
    }

    switch n := node.(type) {
    case *ast.Headline:
        fmt.Printf("%s[Headline L%d] %s (keyword=%s, priority=%s, tags=%v)\n",
            prefix, n.Level, n.Title, n.Keyword, n.Priority, n.Tags)
        for _, child := range n.Children {
            printNode(child, indent+1)
        }
    case *ast.Paragraph:
        fmt.Printf("%s[Paragraph] %s\n", prefix, n.Content)
    case *ast.Keyword:
        fmt.Printf("%s[Keyword] %s: %s\n", prefix, n.Key, n.Value)
    case *ast.List:
        fmt.Printf("%s[List] ordered=%v, items=%d\n", prefix, n.Ordered, len(n.Items))
        for _, item := range n.Items {
            fmt.Printf("%s  - %s (checkbox=%d)\n", prefix, item.Content, item.Checkbox)
        }
    }
}
```

## Advanced Usage

### With Context (Cancellation Support)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

l := lexer.New(input, lexer.WithContext(ctx))
p := parser.New(l, parser.WithContext(ctx))
doc := p.ParseDocument()

if err := l.Err(); err != nil {
    // Handle cancellation or timeout
}
```

### With Custom Logging

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

l := lexer.New(input, lexer.WithLogger(logger))
p := parser.New(l, parser.WithLogger(logger))
```

### With Input Size Limits

```go
l := lexer.New(input,
    lexer.WithMaxInputSize(1024*1024),  // 1MB max
    lexer.WithMaxLineLength(1000),       // 1000 chars per line max
)

if err := l.Err(); err == lexer.ErrInputTooLarge {
    // Handle oversized input
}
```

## Supported Org-mode Elements

### Block Elements

| Element | Syntax | AST Type |
|---------|--------|----------|
| Headline | `* Title` | `*ast.Headline` |
| TODO/DONE | `* TODO Task` | `Headline.Keyword` |
| Priority | `* [#A] Task` | `Headline.Priority` |
| Tags | `* Title :tag1:tag2:` | `Headline.Tags` |
| Paragraph | Plain text | `*ast.Paragraph` |
| Keyword | `#+KEY: value` | `*ast.Keyword` |
| Code Block | `#+BEGIN_SRC ... #+END_SRC` | `*ast.Block` |
| Quote Block | `#+BEGIN_QUOTE ... #+END_QUOTE` | `*ast.Block` |
| Drawer | `:PROPERTIES: ... :END:` | `*ast.Drawer` |
| Unordered List | `- item` or `+ item` | `*ast.List` |
| Ordered List | `1. item` or `1) item` | `*ast.List` |
| Checkbox | `- [ ]`, `- [X]`, `- [-]` | `ListItem.Checkbox` |
| Table | `\| col1 \| col2 \|` | `*ast.Table` |
| Comment | `# comment` | `*ast.Comment` |

### Inline Elements

| Element | Syntax | Type |
|---------|--------|------|
| Bold | `*bold*` | `InlineBold` |
| Italic | `/italic/` | `InlineItalic` |
| Code | `~code~` | `InlineCode` |
| Verbatim | `=verbatim=` | `InlineVerbatim` |
| Strikethrough | `+strike+` | `InlineStrikethrough` |
| Underline | `_underline_` | `InlineUnderline` |
| Link | `[[url][description]]` | `InlineLink` |

Inline elements support nesting (e.g., `*bold with /italic/*`).

### Nested Lists

Indented list items are automatically nested:

```org
- Item 1
  - Nested item 1.1
  - Nested item 1.2
- Item 2
```

## AST Structure

```
Document
├── Keyword (#+TITLE, #+AUTHOR, etc.)
├── Headline (level 1)
│   ├── Drawer (:PROPERTIES:)
│   ├── Paragraph
│   │   └── Inline elements (bold, italic, links, etc.)
│   ├── List
│   │   └── ListItem
│   │       └── Nested List
│   ├── Block (#+BEGIN_SRC)
│   ├── Table
│   └── Headline (level 2, nested)
│       └── ...
└── ...
```

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test ./... -race

# Run with coverage
go test ./... -cover

# Run benchmarks
go test ./... -bench=.
```

## Benchmarks

On Apple M4:

| Benchmark | Time | Allocations |
|-----------|------|-------------|
| Simple Document | ~2.5μs | 99 allocs |
| Complex Document | ~10.7μs | 319 allocs |
| Large Document (100 headlines) | ~195μs | 6672 allocs |

## License

MIT
