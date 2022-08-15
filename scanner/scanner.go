package scanner

import (
	"bufio"
	"errors"
	"log"
	"os"
)

const TAB = 8
const CURLY_COMMENT = 1 // { }
const SLASH_COMMENT = 2 // //
const STAR1_COMMENT = 3 // (* *)
const STAR2_COMMENT = 4 // /* */
const ARROW_ASSG = 1    // <-
const COLON_ASSG = 2    // :=
const NORM_ASSG = 3     // =

var COMMENT = []byte("_COMMENT_")
var comments = []string{"", "{ ... }", "// ... EOL", "(* ... *)", "/* ... */"}
var assgs = []string{"", "'<-'", "':='", "'='"}

var ErrMixComment = errors.New("DAP: Different comment styles")
var usedComment = 0

var ErrMixQuote = errors.New("DAP: Different string styles")
var usedQuote = 0 // one quote or double quote

var usedAssg = 0 // <- := or = as assignment

var symbols = map[string]string{
	"program":     "$PROGRAM",
	"dictionary":  "$DICT",
	"declaration": "$DICT",
	"pseudocode":  "$CODE",
	"code":        "$CODE",
	"algorithm":   "$CODE",
	"endprogram":  "$ENDPROG",
	"input":       "$INPUT",
	"output":      "$OUTPUT",
	"while":       "$WHILE",
	"do":          "$DO",
	"endwhile":    "$ENDWHILE",
	"repeat":      "$REPEAT",
	"until":       "$UNTIL",
	"for":         "$FOR",
	"endfor":      "$ENDFOR",
	"if":          "$IF",
	"then":        "$THEN",
	"else":        "$ELSE",
	"elif":        "$ELIF",
	"elseif":      "$ELIF",
	"endif":       "$ENDIF",
	"case":        "$CASE",
	"of":          "$OF",
	"switch":      "$SWITCH",
	"default":     "$DEFAULT",
	"otherwise":   "$DEFAULT",
	"endcase":     "$ENDCASE",
	"endswitch":   "$ENDCASE",
	"div":         "$DIV",
	"mod":         "$MOD",
	"and":         "$AND",
	"or":          "$OR",
	"not":         "$NOT",
	"true":        "$TRUE",
	"false":       "$FALSE",
	"ref":         "$REF",
	"io":          "$REF",
	"var":         "$VAR",
	"variable":    "$VAR",
	"const":       "$CONST",
	"constant":    "$CONST",
	"array":       "$ARRAY",
	"function":    "$FUNC",
	"endfunc":     "$ENDFUNC",
	"procedure":   "$PROC",
	"endproc":     "$ENDPROC",
	"call":        "$CALL",
	"integer":     "$INT",
	"int":         "$INT",
	"real":        "$REAL",
	"float":       "$REAL",
	"character":   "$CHAR",
	"char":        "$CHAR",
	"boolean":     "$BOOL",
	"bool":        "$BOOL",
	"logical":     "$BOOL",
	"string":      "$CHARRAY",
	"local":       "$LOCAL",
	"global":      "$GLOBAL",
	"_COMMENT_":   "$COMMENT",
	"_LINE_":      "$LINE",
	"(":           "$LEFTPAR",
	")":           "$RIGHTPAR",
	"/":           "$DIV",
	"<":           "$LT",
	"<=":          "$LEQ",
	"<>":          "$NEQ",
	"<-":          "$ASSG",
	">":           "$GT",
	">=":          "$GEQ",
	"><":          "$NEQ",
	":=":          "$ASSG",
	":":           "$COLON",
	";":           "$SEMICOLON",
	",":           "$COMMA",
	"*":           "$MULT",
	"**":          "$POWER",
	"==":          "$EQ",
	"=":           "$MEQ",
	"!=":          "$NEQ",
	"!":           "$NOT",
	"&":           "$BAND",
	"&&":          "$AND",
	"|":           "$BOR",
	"||":          "$OR",
	"%":           "$MOD",
	"^":           "$POWER",
	"+":           "$PLUS",
	"-":           "$MINUS",
	"[":           "$LEFTBRACK",
	"]":           "$RIGHTBRACK",
	"..":          "$RANGE",
	"...":         "$RANGE",
	// "←":           "$ASSG",
	"kamus":       "$DICT",
	"deklarasi":   "$DICT",
	"kode":        "$CODE",
	"algoritma":   "$CODE",
	"endprog":     "$ENDPROG",
	"baca":        "$INPUT",
	"tulis":       "$OUTPUT",
	"read":        "$INPUT",
	"write":       "$OUTPUT",
	"print":       "$OUTPUT",
	"divide":      "$DIV",
	"modulo":      "$MOD",
	"dan":         "$AND",
	"atau":        "$OR",
	"tidak":       "$NOT",
	"benar":       "$TRUE",
	"salah":       "$FALSE",
	"inpout":      "$REF",
	"aray":        "$ARRAY",
	"fungsi":      "$FUNC",
	"endfungsi":   "$ENDFUNC",
	"prosedur":    "$PROC",
	"endprosedur": "$ENDPROC",
	"variabel":    "$VAR",
	"konstan":     "$CONST",
	"lokal":       "$LOCAL",
	"umum":        "$GLOBAL",
}

var lastcol = 0
var colno = 0
var lineno = 0
var comment = 0
var errcount = 0

var linecmt = 0

func checkCommentStyle(comment int) {
	if usedComment == 0 {
		usedComment = comment
		linecmt = lineno
	} else if usedComment != comment {
		log.Printf("DAP.s %v:%v -- Don't mix '%s' vs. '%s' (see line %v)", lineno, colno, comments[usedComment], comments[comment], linecmt)
		errcount++
	}
}

var linestr = 0

func checkStringStyle(quote int) {
	if usedQuote == 0 {
		usedQuote = quote
		linestr = lineno
	} else if usedQuote != quote {
		log.Printf("DAP.s %v:%v -- Don't mix string styles, [\"] vs. ['] (see line %v)", lineno, colno, linestr)
		errcount++
	}
}

var lineasg = 0

func checkAssgStyle(assg int) {
	if usedAssg == 0 {
		usedAssg = assg
		lineasg = lineno
	} else if usedAssg != assg {
		log.Printf("DAP.s %v:%v -- Don't mix assignment styles, %s vs. %s (see line %v)", lineno, colno, assgs[usedAssg], assgs[assg], lineasg)
		errcount++
	}
}

func isEOL(data byte) bool {
	return data == '\r' || data == '\n'
}

func isWhitespace(data byte) bool {
	return data == ' ' || data == '\t'
}

func isLetter(data byte) bool {
	return data == '_' || (data >= '@' && data <= 'Z') || (data >= 'a' && data <= 'z')
}

func isDigit(data byte) bool {
	return data >= '0' && data <= '9'
}

func isLetterDigit(data byte) bool {
	return (data == '_') || (data >= '0' && data <= '9') || (data >= '@' && data <= 'Z') || (data >= 'a' && data <= 'z')
}

func isDot(data []byte) bool {
	return data[0] == '.' && (len(data) == 1 || data[1] != '.')
}

func tokenString(data []byte, atEOF bool) (advance int, token []byte, err error) {
	advance = 0
	token = nil
	err = nil
	colno = lastcol

	// reaching end of file
	if atEOF {
		err = bufio.ErrFinalToken
		return
	}

	skip := 0
	len := len(data)

	// comment area, skipping characters
	if comment > 0 {
		switch comment {
		case CURLY_COMMENT:
			for ; skip < len && !isEOL(data[skip]) && data[skip] != '}'; skip++ {
			}
			if skip < len && data[skip] == '}' {
				skip++
				comment = 0
			}

		case SLASH_COMMENT:
			for ; skip < len && !isEOL(data[skip]); skip++ {
			}
			comment = 0

		case STAR1_COMMENT, STAR2_COMMENT:
			endstarcomment := false
			for ; skip < len && !isEOL(data[skip]) && !endstarcomment; skip++ {
				if data[skip] == '*' {
					if skip+1 < len && (data[skip+1] == '/' || data[skip+1] == ')') {
						endstarcomment = true
						comment = 0
						skip++
					}
				}
			}
		}
	}

	// after skipping whitespaces, collect a token
	for ; skip < len && isWhitespace(data[skip]); skip++ {
	}

	// newline was found or file started, count spaces
	if (skip < len && isEOL(data[skip])) || lineno == 0 {
		colno = 0
		lineno++
		for ; skip < len && data[skip] == '\r'; skip++ {
		}
		if skip < len && data[skip] == '\n' {
			skip++
		}
		spaces := 0
		for ; skip < len && isWhitespace(data[skip]); skip++ {
			if data[skip] == ' ' {
				spaces++
			} else { // tab == every 8th pos
				spaces = ((spaces + TAB) / TAB) * TAB
			}
		}
		advance = skip
		token = []byte("_LINE_")
		lastcol = spaces
		return
	}

	if skip >= len {
		lastcol = skip
		return
	}
	tstart := skip
	colno += tstart
	switch {
	case isLetter(data[skip]):
		for skip++; skip < len && isLetterDigit(data[skip]); skip++ {
		}
		advance = skip
		token = data[tstart:skip]

	case isDigit(data[skip]), isDot(data[skip:]), data[skip] == '-': // num or -
		realnum := data[skip] == '.'
		skip++
		if skip < len && (isDigit(data[skip]) || data[skip] == '.') {
			for ; skip < len && isDigit(data[skip]); skip++ {
			}
			if !realnum && data[skip] == '.' {
				for skip++; skip < len && isDigit(data[skip]); skip++ {
				}
			}
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '{': // { or (* or // or /*
		comment = CURLY_COMMENT
		advance = skip
		token = []byte("_COMMENT_")
		checkCommentStyle(comment)

	case data[skip] == '/': // / or // or /*
		skip++
		advance = skip
		token = data[skip-1 : skip]

		if skip >= len {
		} else if data[skip] == '/' {
			comment = SLASH_COMMENT
			advance = skip + 1
			token = []byte("_COMMENT_")
			checkCommentStyle(comment)
		} else if data[skip] == '*' {
			comment = STAR2_COMMENT
			advance = skip + 1
			token = []byte("_COMMENT_")
			checkCommentStyle(comment)
		}

	case data[skip] == '(': // ( (*
		skip++
		if skip < len && data[skip] == '*' {
			comment = STAR1_COMMENT
			advance = skip + 1
			token = COMMENT
			checkCommentStyle(comment)
		} else {
			advance = skip
			token = data[skip-1 : skip]
		}

	case data[skip] == '"': // "string ""
		for skip++; skip < len && data[skip] != '"'; skip++ {
		}
		if skip < len {
			skip++
		}
		advance = skip
		token = data[tstart : skip-1]
		checkStringStyle('"')

	case data[skip] == '\'': // "string '"
		for skip++; skip < len && data[skip] != '\''; skip++ {
		}
		if skip < len {
			skip++
		}
		advance = skip
		token = data[tstart : skip-1]
		checkStringStyle('\'')

		/*
			case data[skip] == '“': // "string “"
				for skip++; skip < len && data[skip] != '"'; skip++ {
				}
				if skip < len {
					skip++
				}
				advance = skip
				token = data[tstart : skip-1]
				checkStringStyle('“')
		*/
	case data[skip] == '<': // <- <= <> <
		skip++
		if skip >= len {
		} else if data[skip] == '-' { // context sensitive, assgn struct only
			checkAssgStyle(ARROW_ASSG)
			skip++
		} else if data[skip] == '=' || data[skip] == '>' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '>': // >= >< >
		skip++
		if skip >= len {
		} else if data[skip] == '=' || data[skip] == '<' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == ':': // := :
		skip++
		if skip >= len {
		} else if data[skip] == '=' {
			checkAssgStyle(COLON_ASSG)
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '*': // ** *
		skip++
		if skip >= len {
		} else if data[skip] == '*' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '=': // == =
		skip++
		if skip >= len {
		} else if data[skip] == '=' {
			skip++
		} else {
			// checkAssgStyle(NORM_ASSG)
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '!': // != !
		skip++
		if skip >= len {
		} else if data[skip] == '=' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '&': // && &
		skip++
		if skip >= len {
		} else if data[skip] == '&' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '|': // || |
		skip++
		if skip >= len {
		} else if data[skip] == '|' {
			skip++
		}
		advance = skip
		token = data[tstart:skip]

	case data[skip] == '.': // . .. ...
		skip++
		if skip >= len {
		} else if data[skip] == '.' {
			skip++
			if skip >= len {
			} else if data[skip] == '.' {
				skip++
			}
		}
		advance = skip
		token = data[tstart:skip]

	default: // just one character token: % ) ^ , + - [ ]
		skip++
		advance = skip
		token = data[tstart:skip]
	}
	lastcol += skip
	return
}

var scanner *bufio.Scanner

type Token struct {
	Typ, Val string
	Grp      string
	Lno, Cno int
	First    bool
}

var keyUse = map[string]Token{}

func checkKeyUse(t Token) {
	if keyUse[t.Typ].Val == "" {
		keyUse[t.Typ] = Token{Val: t.Val, Lno: t.Lno, Cno: t.Cno}
	} else if keyUse[t.Typ].Val != t.Val {
		log.Printf("DAP.s %v:%v -- Inconsistence keywords: %v vs. %v (see line %v)", lineno, colno, keyUse[t.Typ].Val, t.Val, keyUse[t.Typ].Lno)
		// errcount++
	}
}

func NewToken(dapSrcFile string) *Token {
	file, err := os.Open(dapSrcFile)
	if err != nil {
		panic(err)
	}
	// defer file.Close()
	scanner = bufio.NewScanner(file)
	scanner.Split(tokenString)
	t := new(Token)
	t.Val = "VALUE"
	t.Typ = "TYPE"
	t.Lno = 9999
	t.Cno = 8888
	return t
}

var pushback = false

func (t Token) PushBack() {
	pushback = true
}

/* don't return $LINE, but keep its last value
 */
var lastLine Token

func (t *Token) Next() string {
	if pushback {
		pushback = false
	} else {
		avail := scanner.Scan()
		newline := false

		for avail && (scanner.Text() == "_LINE_" || scanner.Text() == "_COMMENT_") {
			if scanner.Text() == "_LINE_" {
				lastLine.Lno = lineno
				lastLine.Cno = lastcol
				newline = true
			}
			avail = scanner.Scan()
		}
		/*
			for avail  && scanner.Text() == "_LINE_" {
				lastLine.Lno = lineno
				lastLine.Cno = lastcol
				newline = true
				avail = scanner.Scan()
			}
		*/
		if avail {
			t.Val = scanner.Text()
			t.Typ = symbols[t.Val]
			t.Lno = lineno
			t.Cno = colno
			t.First = newline
			// log.Print("tok ", t.Val, t.Typ)
			if t.Typ != "" {
				checkKeyUse(*t)
			} else if t.Val == "" {
				t.Typ = "$ENDPROG"
				t.Val = "EMPTY PROGRAM"
			} else if isLetter(t.Val[0]) {
				t.Typ = "$NAME"
			} else if isDigit(t.Val[0]) || t.Val[0] == '.' || t.Val[0] == '-' {
				t.Typ = "$NUMBER"
			} else {
				t.Typ = "$CHARRAY"
				// log.Print("string=", t.Val, len(t.Val))
			}
		} else {
			t.Typ = "$ENDPROG"
			t.Val = "END OF FILE"
			t.First = true
		}
	}
	return t.Typ
}
func (t *Token) Peek() string {
	t.Next()
	t.PushBack()
	return t.Typ
}

func (t *Token) IsAvail() bool {
	avail := scanner.Scan()
	if avail {
		t.Val = scanner.Text()
		t.Typ = symbols[t.Val]
		t.Lno = lineno
		if t.Typ == "$LINE" {
			t.Cno = lastcol
		} else {
			t.Cno = colno
		}
	}
	return avail
}

func (t Token) Text() string {
	return t.Val
}

func (t Token) Err() error {
	return scanner.Err()
}

func (t Token) ErrCount() int {
	return errcount
}

func (t Token) GetLineCol() (int, int) {
	return lastLine.Lno, lastLine.Cno
}

func (t Token) GetLine() int {
	return lastLine.Lno
}
