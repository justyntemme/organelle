package parser

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/justyntemme/organelle/ast"
	"github.com/justyntemme/organelle/lexer"
	"github.com/justyntemme/organelle/token"
)

var (
	priorityRegex   = regexp.MustCompile(`^\[#([A-Z])\]\s*`)
	tagsRegex       = regexp.MustCompile(`\s+:([a-zA-Z0-9_@#%:]+):\s*$`)
	timestampRegex  = regexp.MustCompile(`[<\[](\d{4}-\d{2}-\d{2})(?:\s+[A-Za-z]+)?(?:\s+(\d{1,2}:\d{2}))?(?:\s+(\+\+?|\.?\+)(\d+[hdwmy]))?(?:\s+(-\d+[hdwmy]))?[>\]]`)
	linkRegex       = regexp.MustCompile(`\[\[([^\]]+)\](?:\[([^\]]+)\])?\]`)
	checkboxRegex   = regexp.MustCompile(`^\s*\[([ X\-])\]\s*`)
	propertyRegex   = regexp.MustCompile(`^:([^:]+):\s*(.*)$`)
)

type Parser struct {
	l         *lexer.Lexer
	curToken  token.Token
	peekToken token.Token
	errors    []string
	logger    *slog.Logger
	ctx       context.Context
}

// Option is a functional option for configuring the Parser
type Option func(*Parser)

// WithLogger sets a custom logger for the parser
func WithLogger(logger *slog.Logger) Option {
	return func(p *Parser) {
		p.logger = logger
	}
}

// WithContext sets a context for cancellation support
func WithContext(ctx context.Context) Option {
	return func(p *Parser) {
		p.ctx = ctx
	}
}

func New(l *lexer.Lexer, opts ...Option) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
		logger: slog.Default(),
		ctx:    context.Background(),
	}

	for _, opt := range opts {
		opt(p)
	}

	// Check for lexer errors
	if err := l.Err(); err != nil {
		p.errors = append(p.errors, err.Error())
	}

	// Read two tokens so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	p.logger.Debug("parser initialized")
	return p
}

// checkContext checks if the context has been cancelled
func (p *Parser) checkContext() bool {
	select {
	case <-p.ctx.Done():
		p.addError("parsing cancelled: %v", p.ctx.Err())
		return true
	default:
		return false
	}
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) addError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	err := fmt.Sprintf("line %d: %s", p.curToken.Line, msg)
	p.errors = append(p.errors, err)
	p.logger.Error("parse error", "line", p.curToken.Line, "message", msg)
}

func (p *Parser) ParseDocument() *ast.Document {
	doc := &ast.Document{}
	doc.Children = []ast.Node{}

	p.logger.Debug("starting document parse")

	// We use a stack to manage headline nesting.
	var stack []*ast.Headline

	for p.curToken.Type != token.EOF {
		// Check for context cancellation periodically
		if p.checkContext() {
			break
		}

		// Check for lexer errors
		if err := p.l.Err(); err != nil {
			p.addError("lexer error: %v", err)
			break
		}

		node := p.parseNode()
		if node != nil {
			if hl, ok := node.(*ast.Headline); ok {
				// Pop stack until we find a parent with level < current level
				for len(stack) > 0 {
					top := stack[len(stack)-1]
					if top.Level < hl.Level {
						break
					}
					stack = stack[:len(stack)-1]
				}

				if len(stack) == 0 {
					doc.Children = append(doc.Children, hl)
				} else {
					parent := stack[len(stack)-1]
					parent.Children = append(parent.Children, hl)
				}

				stack = append(stack, hl)
			} else {
				// Non-headline elements
				if len(stack) > 0 {
					parent := stack[len(stack)-1]
					parent.Children = append(parent.Children, node)
				} else {
					doc.Children = append(doc.Children, node)
				}
			}
		}
		p.nextToken()
	}

	p.logger.Debug("document parse complete", "children", len(doc.Children), "errors", len(p.errors))
	return doc
}

func (p *Parser) parseNode() ast.Node {
	p.logger.Debug("parsing node", "token_type", p.curToken.Type, "line", p.curToken.Line)

	switch p.curToken.Type {
	case token.STARS:
		return p.parseHeadline()
	case token.KEYWORD:
		return p.parseKeyword()
	case token.BLOCK_BEGIN:
		return p.parseBlock()
	case token.DRAWER_BEGIN:
		return p.parseDrawer()
	case token.LIST_ITEM:
		return p.parseList()
	case token.TABLE_ROW, token.TABLE_SEP:
		return p.parseTable()
	case token.COMMENT:
		return p.parseComment()
	case token.TEXT:
		return p.parseParagraph()
	case token.NEWLINE:
		return nil
	default:
		return nil
	}
}

func (p *Parser) parseHeadline() *ast.Headline {
	hl := &ast.Headline{
		Token:    p.curToken,
		Level:    len(p.curToken.Literal),
		Children: []ast.Node{},
	}

	if p.peekTokenIs(token.TEXT) {
		p.nextToken()
		text := strings.TrimSpace(p.curToken.Literal)

		// Extract tags first (they're at the end)
		if matches := tagsRegex.FindStringSubmatch(text); matches != nil {
			tagStr := matches[1]
			hl.Tags = strings.Split(tagStr, ":")
			text = strings.TrimSpace(text[:len(text)-len(matches[0])])
		}

		// Check for TODO/DONE keywords
		if strings.HasPrefix(text, "TODO ") {
			hl.Keyword = "TODO"
			text = strings.TrimSpace(text[5:])
		} else if strings.HasPrefix(text, "DONE ") {
			hl.Keyword = "DONE"
			text = strings.TrimSpace(text[5:])
		} else if text == "TODO" {
			hl.Keyword = "TODO"
			text = ""
		} else if text == "DONE" {
			hl.Keyword = "DONE"
			text = ""
		}

		// Check for priority [#A]
		if matches := priorityRegex.FindStringSubmatch(text); matches != nil {
			hl.Priority = matches[1]
			text = strings.TrimSpace(text[len(matches[0]):])
		}

		hl.Title = text
	}

	p.logger.Debug("parsed headline", "level", hl.Level, "title", hl.Title, "keyword", hl.Keyword, "tags", hl.Tags)
	return hl
}

func (p *Parser) parseKeyword() *ast.Keyword {
	literal := p.curToken.Literal

	if !strings.HasPrefix(literal, "#+") {
		p.addError("invalid keyword format: expected #+KEY: VALUE, got %q", literal)
		return nil
	}

	parts := strings.SplitN(literal, ":", 2)
	key := strings.TrimPrefix(parts[0], "#+")

	if key == "" {
		p.addError("empty keyword key in %q", literal)
		return nil
	}

	val := ""
	if len(parts) > 1 {
		val = strings.TrimSpace(parts[1])
	}

	kw := &ast.Keyword{
		Token: p.curToken,
		Key:   key,
		Value: val,
	}
	p.logger.Debug("parsed keyword", "key", key, "value", val)
	return kw
}

func (p *Parser) parseBlock() *ast.Block {
	block := &ast.Block{
		Token: p.curToken,
	}

	// Parse #+BEGIN_TYPE [LANGUAGE] [PARAMS]
	literal := p.curToken.Literal
	upperLiteral := strings.ToUpper(literal)

	// Extract block type
	typeStart := strings.Index(upperLiteral, "#+BEGIN_") + 8
	rest := literal[typeStart:]
	parts := strings.Fields(rest)

	if len(parts) > 0 {
		block.Type = strings.ToUpper(parts[0])
	}
	if len(parts) > 1 {
		block.Language = parts[1]
	}
	if len(parts) > 2 {
		block.Params = strings.Join(parts[2:], " ")
	}

	// Collect content until #+END_TYPE
	var contentLines []string
	endMarker := "#+END_" + block.Type

	p.nextToken() // Move past BEGIN line
	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.NEWLINE {
			p.nextToken()
			continue
		}
		if p.curToken.Type == token.BLOCK_END {
			upperCur := strings.ToUpper(p.curToken.Literal)
			if strings.HasPrefix(upperCur, endMarker) {
				break
			}
		}
		contentLines = append(contentLines, p.curToken.Literal)
		p.nextToken()
	}

	block.Content = strings.Join(contentLines, "\n")
	p.logger.Debug("parsed block", "type", block.Type, "language", block.Language, "content_lines", len(contentLines))
	return block
}

func (p *Parser) parseDrawer() *ast.Drawer {
	drawer := &ast.Drawer{
		Token:      p.curToken,
		Properties: make(map[string]string),
	}

	// Extract drawer name from :NAME:
	trimmed := strings.TrimSpace(p.curToken.Literal)
	drawer.Name = strings.Trim(trimmed, ":")

	// Collect content until :END:
	var contentLines []string

	p.nextToken() // Move past drawer start
	for p.curToken.Type != token.EOF {
		if p.curToken.Type == token.NEWLINE {
			p.nextToken()
			continue
		}
		if p.curToken.Type == token.DRAWER_END {
			break
		}

		line := p.curToken.Literal

		// If this is a PROPERTIES drawer, parse properties
		if drawer.Name == "PROPERTIES" {
			if matches := propertyRegex.FindStringSubmatch(strings.TrimSpace(line)); matches != nil {
				drawer.Properties[matches[1]] = matches[2]
			}
		} else {
			contentLines = append(contentLines, line)
		}
		p.nextToken()
	}

	drawer.Content = strings.Join(contentLines, "\n")
	p.logger.Debug("parsed drawer", "name", drawer.Name, "properties", len(drawer.Properties))
	return drawer
}

func (p *Parser) parseList() *ast.List {
	list := &ast.List{
		Token: p.curToken,
		Items: []*ast.ListItem{},
	}

	// Determine if ordered by checking the first item
	literal := p.curToken.Literal
	trimmed := strings.TrimSpace(literal)
	list.Ordered = len(trimmed) > 0 && trimmed[0] >= '0' && trimmed[0] <= '9'

	// Get base indentation level
	baseIndent := p.getIndentation(p.curToken.Literal)

	// Parse all list items and build nested structure
	var allItems []*ast.ListItem
	for p.curToken.Type == token.LIST_ITEM {
		item := p.parseListItem()
		if item != nil {
			allItems = append(allItems, item)
		}

		// Check if next token is also a list item
		if p.peekToken.Type == token.NEWLINE {
			p.nextToken()
		}
		if p.peekToken.Type != token.LIST_ITEM {
			break
		}
		p.nextToken()
	}

	// Build nested structure based on indentation
	list.Items = p.buildNestedList(allItems, baseIndent)

	p.logger.Debug("parsed list", "ordered", list.Ordered, "items", len(list.Items))
	return list
}

// getIndentation returns the number of leading whitespace characters
func (p *Parser) getIndentation(s string) int {
	indent := 0
	for _, ch := range s {
		if ch == ' ' {
			indent++
		} else if ch == '\t' {
			indent += 2 // treat tab as 2 spaces for indentation comparison
		} else {
			break
		}
	}
	return indent
}

// buildNestedList converts a flat list of items into a nested structure based on indentation
func (p *Parser) buildNestedList(items []*ast.ListItem, baseIndent int) []*ast.ListItem {
	if len(items) == 0 {
		return items
	}

	var result []*ast.ListItem
	var stack []*ast.ListItem

	for _, item := range items {
		// Pop items from stack that are at same or deeper indentation
		for len(stack) > 0 && stack[len(stack)-1].Indent >= item.Indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			// This is a top-level item
			result = append(result, item)
		} else {
			// This is a nested item - add to parent's children
			parent := stack[len(stack)-1]
			// Wrap in a nested list if needed
			var nestedList *ast.List
			// Check if parent already has a nested list as last child
			if len(parent.Children) > 0 {
				if existing, ok := parent.Children[len(parent.Children)-1].(*ast.List); ok {
					nestedList = existing
				}
			}
			if nestedList == nil {
				nestedList = &ast.List{
					Token:   item.Token,
					Ordered: item.Indent > 0 && len(item.Content) > 0 && item.Content[0] >= '0' && item.Content[0] <= '9',
					Items:   []*ast.ListItem{},
				}
				parent.Children = append(parent.Children, nestedList)
			}
			nestedList.Items = append(nestedList.Items, item)
		}

		stack = append(stack, item)
	}

	return result
}

func (p *Parser) parseListItem() *ast.ListItem {
	literal := p.curToken.Literal
	item := &ast.ListItem{
		Token:    p.curToken,
		Indent:   p.getIndentation(literal),
		Checkbox: ast.CheckboxNone,
		Children: []ast.Node{},
	}

	content := strings.TrimSpace(literal)

	// Remove list marker (-, +, 1., 1))
	if strings.HasPrefix(content, "- ") {
		content = content[2:]
	} else if strings.HasPrefix(content, "+ ") {
		content = content[2:]
	} else {
		// Ordered list: remove "N. " or "N) "
		for i, ch := range content {
			if ch == '.' || ch == ')' {
				if i+1 < len(content) && content[i+1] == ' ' {
					content = content[i+2:]
					break
				}
			}
			if ch < '0' || ch > '9' {
				break
			}
		}
	}

	// Check for checkbox
	if matches := checkboxRegex.FindStringSubmatch(content); matches != nil {
		switch matches[1] {
		case " ":
			item.Checkbox = ast.CheckboxUnchecked
		case "X":
			item.Checkbox = ast.CheckboxChecked
		case "-":
			item.Checkbox = ast.CheckboxPartial
		}
		content = strings.TrimSpace(content[len(matches[0]):])
	}

	item.Content = content
	return item
}

func (p *Parser) parseTable() *ast.Table {
	table := &ast.Table{
		Token: p.curToken,
		Rows:  []*ast.TableRow{},
	}

	for p.curToken.Type == token.TABLE_ROW || p.curToken.Type == token.TABLE_SEP {
		row := p.parseTableRow()
		if row != nil {
			table.Rows = append(table.Rows, row)
		}

		if p.peekToken.Type == token.NEWLINE {
			p.nextToken()
		}
		if p.peekToken.Type != token.TABLE_ROW && p.peekToken.Type != token.TABLE_SEP {
			break
		}
		p.nextToken()
	}

	p.logger.Debug("parsed table", "rows", len(table.Rows))
	return table
}

func (p *Parser) parseTableRow() *ast.TableRow {
	row := &ast.TableRow{
		Token:     p.curToken,
		Separator: p.curToken.Type == token.TABLE_SEP,
	}

	if !row.Separator {
		// Parse cells
		literal := strings.TrimSpace(p.curToken.Literal)
		literal = strings.Trim(literal, "|")
		cells := strings.Split(literal, "|")
		for _, cell := range cells {
			row.Cells = append(row.Cells, strings.TrimSpace(cell))
		}
	}

	return row
}

func (p *Parser) parseComment() *ast.Comment {
	comment := &ast.Comment{
		Token: p.curToken,
	}

	literal := p.curToken.Literal
	if strings.HasPrefix(literal, "# ") {
		comment.Content = literal[2:]
	} else if literal == "#" {
		comment.Content = ""
	} else {
		comment.Content = strings.TrimPrefix(literal, "#")
	}

	p.logger.Debug("parsed comment", "content", comment.Content)
	return comment
}

func (p *Parser) parseParagraph() *ast.Paragraph {
	para := &ast.Paragraph{
		Token:   p.curToken,
		Content: p.curToken.Literal,
	}

	// Parse inline elements
	para.Inline = p.parseInlineElements(para.Content)

	return para
}

// inlineMarkers maps opening markers to their type and closing marker
var inlineMarkers = map[byte]struct {
	typ     ast.InlineType
	closer  byte
	nestable bool // whether content can contain nested formatting
}{
	'*': {ast.InlineBold, '*', true},
	'/': {ast.InlineItalic, '/', true},
	'~': {ast.InlineCode, '~', false},          // code is not nestable
	'=': {ast.InlineVerbatim, '=', false},      // verbatim is not nestable
	'+': {ast.InlineStrikethrough, '+', true},
	'_': {ast.InlineUnderline, '_', true},
}

func (p *Parser) parseInlineElements(text string) []ast.InlineElement {
	return p.parseInlineElementsRecursive(text, 0)
}

// parseInlineElementsRecursive parses inline elements with support for nesting
// depth is used to prevent infinite recursion
func (p *Parser) parseInlineElementsRecursive(text string, depth int) []ast.InlineElement {
	const maxDepth = 10 // prevent infinite recursion on malformed input
	if depth > maxDepth {
		return []ast.InlineElement{{Type: ast.InlineText, Content: text}}
	}

	var elements []ast.InlineElement
	remaining := text

	for len(remaining) > 0 {
		// Check for links [[url][desc]] first
		if len(remaining) > 2 && remaining[0] == '[' && remaining[1] == '[' {
			if matches := linkRegex.FindStringSubmatchIndex(remaining); matches != nil && matches[0] == 0 {
				url := remaining[matches[2]:matches[3]]
				desc := ""
				if matches[4] != -1 {
					desc = remaining[matches[4]:matches[5]]
				}
				elem := ast.InlineElement{
					Type: ast.InlineLink,
					URL:  url,
				}
				// Parse description for nested formatting
				if desc != "" {
					elem.Children = p.parseInlineElementsRecursive(desc, depth+1)
				}
				elements = append(elements, elem)
				remaining = remaining[matches[1]:]
				continue
			}
		}

		// Check for inline formatting markers
		if marker, ok := inlineMarkers[remaining[0]]; ok && len(remaining) > 2 {
			// Find the closing marker
			end := p.findClosingMarker(remaining[1:], marker.closer)
			if end != -1 && end > 0 {
				innerContent := remaining[1 : end+1]
				elem := ast.InlineElement{Type: marker.typ}

				if marker.nestable {
					// Recursively parse inner content for nested formatting
					elem.Children = p.parseInlineElementsRecursive(innerContent, depth+1)
				} else {
					// Non-nestable (code, verbatim) - store as raw content
					elem.Content = innerContent
				}

				elements = append(elements, elem)
				remaining = remaining[end+2:]
				continue
			}
		}

		// Find next potential marker
		nextMarker := p.findNextMarker(remaining)
		if nextMarker == -1 {
			// No more markers, rest is plain text
			elements = append(elements, ast.InlineElement{
				Type:    ast.InlineText,
				Content: remaining,
			})
			break
		} else if nextMarker > 0 {
			// Plain text before the marker
			elements = append(elements, ast.InlineElement{
				Type:    ast.InlineText,
				Content: remaining[:nextMarker],
			})
			remaining = remaining[nextMarker:]
		} else {
			// Marker at start but didn't match a valid pattern, consume as text
			elements = append(elements, ast.InlineElement{
				Type:    ast.InlineText,
				Content: string(remaining[0]),
			})
			remaining = remaining[1:]
		}
	}

	return elements
}

// findClosingMarker finds the position of the closing marker, respecting nesting
func (p *Parser) findClosingMarker(text string, closer byte) int {
	for i := 0; i < len(text); i++ {
		if text[i] == closer {
			return i
		}
	}
	return -1
}

// findNextMarker finds the position of the next potential inline marker
func (p *Parser) findNextMarker(text string) int {
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '*' || ch == '/' || ch == '~' || ch == '=' || ch == '+' || ch == '_' {
			return i
		}
		if ch == '[' && i+1 < len(text) && text[i+1] == '[' {
			return i
		}
	}
	return -1
}

func (p *Parser) peekTokenIs(t token.TokenType) bool {
	return p.peekToken.Type == t
}

// ParseTimestamp parses a timestamp string and returns a Timestamp node
func ParseTimestamp(text string) *ast.Timestamp {
	matches := timestampRegex.FindStringSubmatch(text)
	if matches == nil {
		return nil
	}

	ts := &ast.Timestamp{
		Active: strings.HasPrefix(text, "<"),
		Date:   matches[1],
	}

	if len(matches) > 2 && matches[2] != "" {
		ts.Time = matches[2]
	}
	if len(matches) > 4 && matches[4] != "" {
		ts.Repeat = matches[3] + matches[4]
	}
	if len(matches) > 5 && matches[5] != "" {
		ts.Warning = matches[5]
	}

	return ts
}
