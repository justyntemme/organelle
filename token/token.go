package token

type TokenType string

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int // Added for better error reporting
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	// Org Mode Elements
	STARS       = "STARS"       // * or ** or ***
	KEYWORD     = "KEYWORD"     // #+TITLE:
	TEXT        = "TEXT"        // Regular content
	NEWLINE     = "NEWLINE"     // \n
	TODO        = "TODO"        // TODO keyword
	DONE        = "DONE"        // DONE keyword
	PRIORITY    = "PRIORITY"    // [#A]
	BLOCK_BEGIN = "BLOCK_BEGIN" // #+BEGIN_SRC, #+BEGIN_QUOTE, etc.
	BLOCK_END   = "BLOCK_END"   // #+END_SRC, #+END_QUOTE, etc.
	DRAWER_BEGIN = "DRAWER_BEGIN" // :PROPERTIES:
	DRAWER_END   = "DRAWER_END"   // :END:
	LIST_ITEM   = "LIST_ITEM"   // - or + or 1. or 1)
	TABLE_ROW   = "TABLE_ROW"   // | col1 | col2 |
	TABLE_SEP   = "TABLE_SEP"   // |---+---|
	TIMESTAMP   = "TIMESTAMP"   // <2024-01-01> or [2024-01-01]
	LINK        = "LINK"        // [[url][description]]
	COMMENT     = "COMMENT"     // # comment
)

// LookupIdent checks if a text might be a specific keyword
func LookupIdent(ident string) TokenType {
	switch ident {
	case "TODO":
		return TODO
	case "DONE":
		return DONE
	default:
		return TEXT
	}
}
