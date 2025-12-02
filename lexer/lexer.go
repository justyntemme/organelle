package lexer

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/justyntemme/organelle/token"
)

// Default limits for input validation
const (
	DefaultMaxInputSize = 10 * 1024 * 1024 // 10MB
	DefaultMaxLineLength = 10000            // 10K characters per line
)

// ErrInputTooLarge is returned when input exceeds the maximum size
var ErrInputTooLarge = errors.New("input exceeds maximum allowed size")

// ErrLineTooLong is returned when a line exceeds the maximum length
var ErrLineTooLong = errors.New("line exceeds maximum allowed length")

// Lexer follows the standard Rob Pike style state handling, adapted for
// struct-based iteration for easier integration with the parser.
type Lexer struct {
	input          string
	position       int  // current position in input (points to current char)
	readPosition   int  // current reading position in input (after current char)
	ch             rune // current char under examination
	prevCh         rune // previous character for line-start detection
	line           int  // line number for error reporting
	column         int  // column number for error reporting
	logger         *slog.Logger
	ctx            context.Context
	maxInputSize   int
	maxLineLength  int
	err            error // stores any error encountered during lexing
}

// Option is a functional option for configuring the Lexer
type Option func(*Lexer)

// WithLogger sets a custom logger for the lexer
func WithLogger(logger *slog.Logger) Option {
	return func(l *Lexer) {
		l.logger = logger
	}
}

// WithContext sets a context for cancellation support
func WithContext(ctx context.Context) Option {
	return func(l *Lexer) {
		l.ctx = ctx
	}
}

// WithMaxInputSize sets the maximum allowed input size in bytes
func WithMaxInputSize(size int) Option {
	return func(l *Lexer) {
		l.maxInputSize = size
	}
}

// WithMaxLineLength sets the maximum allowed line length in characters
func WithMaxLineLength(length int) Option {
	return func(l *Lexer) {
		l.maxLineLength = length
	}
}

// New creates a new Lexer with the given input and options
func New(input string, opts ...Option) *Lexer {
	l := &Lexer{
		input:         input,
		line:          1,
		column:        0,
		logger:        slog.Default(),
		ctx:           context.Background(),
		maxInputSize:  DefaultMaxInputSize,
		maxLineLength: DefaultMaxLineLength,
	}

	for _, opt := range opts {
		opt(l)
	}

	// Validate input size
	if len(input) > l.maxInputSize {
		l.err = ErrInputTooLarge
		l.logger.Error("input too large", "size", len(input), "max", l.maxInputSize)
	}

	l.logger.Debug("lexer initialized", "input_length", len(input))
	l.readChar()
	return l
}

// Err returns any error encountered during lexing
func (l *Lexer) Err() error {
	return l.err
}

// checkContext checks if the context has been cancelled
func (l *Lexer) checkContext() bool {
	select {
	case <-l.ctx.Done():
		l.err = l.ctx.Err()
		return true
	default:
		return false
	}
}

func (l *Lexer) readChar() {
	l.prevCh = l.ch
	if l.readPosition >= len(l.input) {
		l.ch = 0
		l.position = l.readPosition
		l.readPosition++
		l.column++
	} else {
		r, w := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.position = l.readPosition
		l.readPosition += w
		if l.prevCh == '\n' {
			l.column = 1
		} else {
			l.column++
		}
	}
}

func (l *Lexer) peekChar() rune {
	if l.readPosition >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.readPosition:])
	return r
}

// peekString looks ahead n bytes and returns the string
func (l *Lexer) peekString(n int) string {
	end := l.readPosition + n
	if end > len(l.input) {
		end = len(l.input)
	}
	return l.input[l.readPosition:end]
}

// NextToken returns the next token from the input
func (l *Lexer) NextToken() token.Token {
	var tok token.Token
	tok.Line = l.line
	tok.Column = l.column

	// Check for errors or cancellation
	if l.err != nil {
		tok.Type = token.EOF
		tok.Literal = ""
		return tok
	}

	// Check context cancellation periodically (every token)
	if l.checkContext() {
		tok.Type = token.EOF
		tok.Literal = ""
		return tok
	}

	// Check for line-start specific tokens (Headlines, Keywords)
	isLineStart := l.position == 0 || l.prevCh == '\n'

	switch l.ch {
	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
		l.logger.Debug("token", "type", tok.Type, "line", tok.Line)
		return tok

	case '\n':
		tok = l.newToken(token.NEWLINE, l.ch)
		l.line++
		l.readChar()
		return tok

	case '*':
		if isLineStart {
			stars := l.readStars()
			if l.ch == ' ' {
				tok.Type = token.STARS
				tok.Literal = stars
				l.logger.Debug("token", "type", tok.Type, "literal", tok.Literal, "line", tok.Line)
				return tok
			}
			// Not a headline, treat as text
			tok.Type = token.TEXT
			tok.Literal = stars + l.readToEndOfLine()
			l.logger.Debug("token", "type", tok.Type, "line", tok.Line)
			return tok
		}
		tok = l.readTextLine()
		return tok

	case '#':
		if isLineStart {
			peek := l.peekChar()
			if peek == '+' {
				// Could be #+KEYWORD or #+BEGIN/#+END
				tok = l.readOrgDirective()
				return tok
			} else if peek == ' ' || peek == '\n' || peek == 0 {
				// Comment line: # comment
				tok = l.readComment()
				return tok
			}
		}
		tok = l.readTextLine()
		return tok

	case ':':
		if isLineStart {
			// Could be a drawer :NAME: or property :KEY: VALUE
			tok = l.readDrawerOrProperty()
			return tok
		}
		tok = l.readTextLine()
		return tok

	case '-':
		if isLineStart {
			// Could be list item "- item" or horizontal rule "-----"
			tok = l.readDashLine()
			return tok
		}
		tok = l.readTextLine()
		return tok

	case '+':
		if isLineStart && l.peekChar() == ' ' {
			// List item "+ item"
			tok = l.readListItem()
			return tok
		}
		tok = l.readTextLine()
		return tok

	case '|':
		if isLineStart {
			tok = l.readTableRow()
			return tok
		}
		tok = l.readTextLine()
		return tok

	case ' ', '\t':
		if isLineStart {
			// Could be an indented list item - look ahead
			tok = l.tryReadIndentedListItem()
			if tok.Type != token.ILLEGAL {
				return tok
			}
		}
		tok = l.readTextLine()
		return tok

	default:
		if isLineStart && l.ch >= '0' && l.ch <= '9' {
			// Could be ordered list: 1. or 1)
			tok = l.tryReadOrderedListItem()
			if tok.Type != token.ILLEGAL {
				return tok
			}
		}
		tok = l.readTextLine()
		return tok
	}
}

func (l *Lexer) newToken(tokenType token.TokenType, ch rune) token.Token {
	tok := token.Token{Type: tokenType, Literal: string(ch), Line: l.line, Column: l.column}
	l.logger.Debug("token", "type", tokenType, "literal", string(ch), "line", l.line)
	return tok
}

func (l *Lexer) readStars() string {
	position := l.position
	for l.ch == '*' {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) readToEndOfLine() string {
	position := l.position
	charCount := 0
	for l.ch != '\n' && l.ch != 0 {
		charCount++
		if charCount > l.maxLineLength {
			l.err = ErrLineTooLong
			l.logger.Error("line too long", "line", l.line, "length", charCount, "max", l.maxLineLength)
			break
		}
		l.readChar()
	}
	return l.input[position:l.position]
}

// readOrgDirective handles #+KEYWORD, #+BEGIN_X, #+END_X
func (l *Lexer) readOrgDirective() token.Token {
	position := l.position
	line := l.line
	col := l.column

	// Read until end of line
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]
	upperLiteral := strings.ToUpper(literal)

	// Check for BEGIN/END blocks
	if strings.HasPrefix(upperLiteral, "#+BEGIN_") {
		l.logger.Debug("token", "type", token.BLOCK_BEGIN, "literal", literal, "line", line)
		return token.Token{Type: token.BLOCK_BEGIN, Literal: literal, Line: line, Column: col}
	}
	if strings.HasPrefix(upperLiteral, "#+END_") {
		l.logger.Debug("token", "type", token.BLOCK_END, "literal", literal, "line", line)
		return token.Token{Type: token.BLOCK_END, Literal: literal, Line: line, Column: col}
	}

	l.logger.Debug("token", "type", token.KEYWORD, "literal", literal, "line", line)
	return token.Token{Type: token.KEYWORD, Literal: literal, Line: line, Column: col}
}

// readComment handles # comment lines
func (l *Lexer) readComment() token.Token {
	position := l.position
	line := l.line
	col := l.column

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.COMMENT, "literal", literal, "line", line)
	return token.Token{Type: token.COMMENT, Literal: literal, Line: line, Column: col}
}

// readDrawerOrProperty handles :NAME: lines
func (l *Lexer) readDrawerOrProperty() token.Token {
	position := l.position
	line := l.line
	col := l.column

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]
	trimmed := strings.TrimSpace(literal)

	// Check for :END:
	if strings.ToUpper(trimmed) == ":END:" {
		l.logger.Debug("token", "type", token.DRAWER_END, "literal", literal, "line", line)
		return token.Token{Type: token.DRAWER_END, Literal: literal, Line: line, Column: col}
	}

	// Check for drawer start :NAME: (must be only :NAME: on the line, possibly with whitespace)
	if strings.HasPrefix(trimmed, ":") && strings.HasSuffix(trimmed, ":") && strings.Count(trimmed, ":") == 2 {
		l.logger.Debug("token", "type", token.DRAWER_BEGIN, "literal", literal, "line", line)
		return token.Token{Type: token.DRAWER_BEGIN, Literal: literal, Line: line, Column: col}
	}

	// Otherwise it's text (could be a property inside a drawer, parser will handle)
	l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line)
	return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
}

// readDashLine handles - list items or ----- horizontal rules
func (l *Lexer) readDashLine() token.Token {
	position := l.position
	line := l.line
	col := l.column

	dashCount := 0
	for l.ch == '-' {
		dashCount++
		l.readChar()
	}

	// Horizontal rule: 5+ dashes followed by end of line
	if dashCount >= 5 && (l.ch == '\n' || l.ch == 0) {
		literal := l.input[position:l.position]
		l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line, "note", "horizontal_rule")
		return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
	}

	// List item: - followed by space
	if dashCount == 1 && l.ch == ' ' {
		// Read the rest of the line
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		literal := l.input[position:l.position]
		l.logger.Debug("token", "type", token.LIST_ITEM, "literal", literal, "line", line)
		return token.Token{Type: token.LIST_ITEM, Literal: literal, Line: line, Column: col}
	}

	// Not a list item or rule, read as text
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line)
	return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
}

// readListItem handles + list items
func (l *Lexer) readListItem() token.Token {
	position := l.position
	line := l.line
	col := l.column

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.LIST_ITEM, "literal", literal, "line", line)
	return token.Token{Type: token.LIST_ITEM, Literal: literal, Line: line, Column: col}
}

// tryReadOrderedListItem tries to read ordered list items like 1. or 1)
func (l *Lexer) tryReadOrderedListItem() token.Token {
	position := l.position
	line := l.line
	col := l.column

	// Read digits
	for l.ch >= '0' && l.ch <= '9' {
		l.readChar()
	}

	// Check for . or ) followed by space
	if (l.ch == '.' || l.ch == ')') && l.peekChar() == ' ' {
		l.readChar() // consume . or )
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
		literal := l.input[position:l.position]
		l.logger.Debug("token", "type", token.LIST_ITEM, "literal", literal, "line", line)
		return token.Token{Type: token.LIST_ITEM, Literal: literal, Line: line, Column: col}
	}

	// Not an ordered list, reset and return ILLEGAL to signal caller to read as text
	// We need to continue reading the line as text
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line)
	return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
}

// tryReadIndentedListItem tries to read indented list items (for nested lists)
func (l *Lexer) tryReadIndentedListItem() token.Token {
	position := l.position
	line := l.line
	col := l.column

	// Skip leading whitespace
	for l.ch == ' ' || l.ch == '\t' {
		l.readChar()
	}

	// Check if we have a list marker after the whitespace
	if l.ch == '-' || l.ch == '+' {
		if l.peekChar() == ' ' {
			// This is an indented list item
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			literal := l.input[position:l.position]
			l.logger.Debug("token", "type", token.LIST_ITEM, "literal", literal, "line", line)
			return token.Token{Type: token.LIST_ITEM, Literal: literal, Line: line, Column: col}
		}
	}

	// Check for ordered list marker (digit followed by . or ))
	if l.ch >= '0' && l.ch <= '9' {
		startDigit := l.position
		for l.ch >= '0' && l.ch <= '9' {
			l.readChar()
		}
		if (l.ch == '.' || l.ch == ')') && l.peekChar() == ' ' {
			l.readChar() // consume . or )
			for l.ch != '\n' && l.ch != 0 {
				l.readChar()
			}
			literal := l.input[position:l.position]
			l.logger.Debug("token", "type", token.LIST_ITEM, "literal", literal, "line", line)
			return token.Token{Type: token.LIST_ITEM, Literal: literal, Line: line, Column: col}
		}
		// Not a list, need to continue reading - reset position tracking
		_ = startDigit // unused but keeps track
	}

	// Not a list item, read rest as text
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line)
	return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
}

// readTableRow handles | table | rows |
func (l *Lexer) readTableRow() token.Token {
	position := l.position
	line := l.line
	col := l.column

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]

	// Check if it's a separator row |---+---|
	trimmed := strings.TrimSpace(literal)
	isSeparator := strings.HasPrefix(trimmed, "|") &&
		strings.HasSuffix(trimmed, "|") &&
		!strings.ContainsAny(strings.Trim(trimmed, "|"), "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	if isSeparator && strings.Contains(trimmed, "-") {
		l.logger.Debug("token", "type", token.TABLE_SEP, "literal", literal, "line", line)
		return token.Token{Type: token.TABLE_SEP, Literal: literal, Line: line, Column: col}
	}

	l.logger.Debug("token", "type", token.TABLE_ROW, "literal", literal, "line", line)
	return token.Token{Type: token.TABLE_ROW, Literal: literal, Line: line, Column: col}
}

// readTextLine reads until the next newline
func (l *Lexer) readTextLine() token.Token {
	position := l.position
	line := l.line
	col := l.column

	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}

	literal := l.input[position:l.position]
	l.logger.Debug("token", "type", token.TEXT, "literal", literal, "line", line)
	return token.Token{Type: token.TEXT, Literal: literal, Line: line, Column: col}
}
