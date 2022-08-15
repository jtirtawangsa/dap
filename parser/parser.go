package parser

import (
	"log"
	"strconv"
	"strings"
	em "untel/dap/emulator"
	"untel/dap/scanner"
)

type nameattr struct {
	parent string
	typ    string
	val    string
	loc    int
}

var (
	errcount = 0
	parent   = ""
	varcoll  = map[string]nameattr{}
	loc      = 0
	token    *scanner.Token
)

/* skip until typ is found
   alternate stopping tokens are stop and dead
*/
func sync(typ string, stop string, dead string) {
	for t := token.Next(); t != typ && t != stop && t != dead; t = token.Next() {
		// log.Println(" waiting", typ, "skipping", t)
	}
	token.PushBack()
}

/* skip if typ is found
 */
func skip(typ string) bool {
	// log.Print("skip ", typ)
	matched := token.Next() == typ
	if !matched {
		token.PushBack()
	}
	return matched
}

func expect(tok, msg string) {
	if token.Next() != tok {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- %s", l, c, msg)
		token.PushBack()
		errcount++
	}
}

/*
   variable { , variable }*
*/
func variable_list(gen func(string, string, int)) (namelist []string) {
	// log.Print("variables", token.Val)
	namelist = []string{}
	for {
		if typ := token.Next(); typ == "$NAME" {
			// found a declared name
			namelist = append(namelist, token.Val)
			attr := varcoll[parent+":"+token.Val]
			gen(attr.typ, attr.parent, attr.loc)
			// log.Print(" ", token.Val)
		}
		if token.Next() != "$COMMA" {
			break
		}
	}
	token.PushBack()
	return
}

/*
   { [var] variable_list : type }+
*/
func declaration() {
	// log.Print("declaring ")
	totdecl := 0
	for typ := token.Next(); typ == "$DICT" || typ == "$LOCAL" || typ == "$GLOBAL"; typ = token.Next() {
	}
	for {
		// em.GenLine(token.GetLineCol())
		if token.Typ == "$CONST" {
			skip("$CONST")
			expect("$NAME", "name expected")
			clabel := token.Val
			skip("$MEQ")
			skip("$ASSIGN")
			typ, val := expression()
			varcoll[parent+":"+clabel] = nameattr{parent: parent, typ: typ, val: val} // typ&val from exp
		} else {
			token.PushBack()
			skip("$VAR")
			namelist := variable_list(em.GenNil)
			totdecl += len(namelist)
			skip("$COLON")
			typ := token.Next()
			if typ == "$INT" || typ == "$REAL" || typ == "$CHAR" || typ == "$BOOL" || typ == "$CHARRAY" {
				if typ == "$CHAR" {
					typ = "$CHARRAY" // for now, later will split again
				}
				for _, v := range namelist {
					loc++
					varcoll[parent+":"+v] = nameattr{parent: parent, typ: typ, val: em.EMPTY, loc: loc}
				}
				// log.Print(" ", token.Val)
			} else {
				token.PushBack()
			}
		}
		if typ := token.Peek(); typ == "$CODE" || typ == "$ENDPROG" {
			break
		}
	}
	// log.Println("declared vars: ", totdecl)
	if totdecl > 0 {
		em.GenClaim(totdecl)
	}
	token.PushBack()
	/*
		for k, v := range varcoll {
			log.Println("declared variables", k, v)
		}
	*/
}

func isStartExpression() bool {
	typ := token.Peek()
	return typ == "$NAME" || typ == "$NUMBER" || typ == "$CHAR" || typ == "$CHARRAY" || typ == "$TRUE" || typ == "$FALSE" || typ == "$LEFTPAR" || typ == "$MINUS" || typ == "$NOT"
}

/* value | variable | - expr | par_expr
 */
func literal() (string, string) {
	typ := "$NUMBER"
	val := em.EMPTY
	// log.Print("literal ", token.Peek())
	switch token.Next() {
	case "$NAME":
		attr := varcoll[parent+":"+token.Val]
		typ = attr.typ
		if typ == "$INT" || typ == "$REAL" {
			typ = "$NUMBER"
		}
		if attr == (nameattr{}) {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Error, variable %v:%v is not defined", l, c, parent, token.Val)
			errcount++
		} else if attr.val == em.EMPTY {
			em.GenCopy(parent, attr.loc)
		} else {
			val = attr.val
		}
		// log.Printf("name=%v type=%v %v val=%v %v", token.Val, attr.typ, typ, attr.val, val)

	case "$NUMBER":
		typ = "$NUMBER"
		val = token.Val
		// em.GenConst( token.Val )
		// log.Print("value=", token.Val)

	case "$CHAR", "$CHARRAY":
		typ = "$CHARRAY"
		if v, err := strconv.Unquote(token.Val + string(token.Val[0])); err != nil {
			log.Print(err)
			val = token.Val[1:]
		} else {
			val = v
			// log.Print(v)
		}
		// log.Print("string=", token.Val)
		// em.GenConst( token.Val )

	case "$TRUE":
		typ = "$BOOL"
		val = "true"

	case "$FALSE":
		typ = "$BOOL"
		val = "false"
		// em.GenConst( token.Val )
		// log.Print("boolean=", token.Val)

	case "$LEFTPAR":
		typ, val = expression()
		expect("$RIGHTPAR", "missing )")

	case "$MINUS":
		typ, val = literal()
		if typ != "$NUMBER" {
			l, c := token.GetLineCol()
			log.Printf("DAP.p  %v:%v -- Negation on a non numeric value", l, c)
			errcount++
		} else {
			nval, err := strconv.Atoi(val)
			if err != nil {
				val = em.EMPTY
				em.GenOpCmd("$MINUS")
			} else {
				val = strconv.Itoa(-nval)
			}
		}
		typ = "$NUMBER"

	case "$NOT":
		typ, val = literal()
		if typ != "$BOOL" {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- NOT operation on a non boolean value ", l, c, typ)
			errcount++
		} else if val == em.TRUE {
			val = em.FALSE
		} else if val == em.FALSE {
			val = em.TRUE
		} else {
			em.GenOpCmd("$NOT")
		}
		typ = "$BOOL"
	}
	return typ, val
}

/* literal [relexpr literal]
 */
func relexpr() (string, string) {
	//log.Print("comparison")
	atyp, aval := literal()
	//log.Print("from lit1 ", atyp, aval)
	if typ := token.Peek(); typ == "$LEQ" || typ == "$GT" || typ == "$LT" || typ == "$GEQ" || typ == "$EQ" || typ == "$NEQ" {
		token.Next()
		btyp, bval := literal()
		// log.Print("from lit ", atyp, btyp)
		if atyp != btyp { // only $NUMBER is meaningful at this time
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Mismatch cmp-expression %v vs. %v", l, c, atyp, btyp)
			errcount++
		}
		val := em.EMPTY
		if aval != em.EMPTY && bval != em.EMPTY {
			cmpval := false
			anval, erra := strconv.Atoi(aval)
			bnval, errb := strconv.Atoi(bval)
			if erra == nil && errb == nil {
				switch typ {
				case "$LE":
					cmpval = anval <= bnval
				case "$LT":
					cmpval = anval < bnval
				case "$GE":
					cmpval = anval >= bnval
				case "$GT":
					cmpval = anval > bnval
				case "$EQ":
					cmpval = anval == bnval
				case "$NEQ":
					cmpval = anval != bnval
				}
				val = strconv.FormatBool(cmpval)
			}
		} else {
			em.GenOp2Cmd(typ, atyp, aval, btyp, bval)
		}
		return "$BOOL", val
	}
	return atyp, aval
}

/* literal {addop literal}*
   :D relexpr {mulop relexpr}* !!!
*/
func mulexpr() (string, string) {
	//log.Print("addition")
	atyp := ""
	aval := em.EMPTY
	btyp := ""
	bval := em.EMPTY
	typ := ""
	for {
		btyp, bval = relexpr()
		//log.Print("from relexpr ", atyp, btyp)
		if atyp == "" {
			atyp = btyp
			aval = bval
		} else if atyp != btyp {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Mismatch mul-expression %v vs. %v", l, c, atyp, btyp)
			errcount++
		} else if atyp == "$NUMBER" {
			anval, erra := strconv.Atoi(aval)
			bnval, errb := strconv.Atoi(bval)
			if erra != nil || errb != nil {
				em.GenOp2Cmd(typ, atyp, aval, btyp, bval) // */%
				bval = em.EMPTY
			} else {
				switch typ {
				case "$MULT":
					bval = strconv.Itoa(anval * bnval)
				case "$DIV":
					if bnval != 0 {
						bval = strconv.Itoa(anval / bnval)
					} else {
						bval = em.EMPTY
					}
				case "$MOD":
					if bnval != 0 {
						bval = strconv.Itoa(anval % bnval)
					} else {
						bval = em.EMPTY
					}
				default:
					l, c := token.GetLineCol()
					log.Printf("DAP %v:%v -- Illegal operands on numbers %v", l, c, typ)
					errcount++
				}
			}
		} else if atyp == "$BOOL" {
			if typ == "$AND" {
				if aval == em.EMPTY || bval == em.EMPTY {
					em.GenOp2Cmd(typ, atyp, aval, btyp, bval) // AND
					bval = em.EMPTY
				} else if aval == em.TRUE && bval == em.TRUE {
					bval = em.TRUE
				} else {
					bval = em.FALSE
				}
			} else {
				l, c := token.GetLineCol()
				log.Printf("DAP.p %v:%v -- Illegal operands on boolean %v", l, c, typ)
				errcount++
			}
		} else if atyp == "$CHARRAY" {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Illegal operation %v on %v", l, c, typ, btyp)
			errcount++
		}
		if typ = token.Peek(); typ != "$MULT" && typ != "$DIV" && typ != "$MOD" && typ != "$AND" {
			break
		}
		token.Next()
	}
	return btyp, bval
}

/* addexpr {mulop addexpr}*
   :D mulexpr {addop mulexpr}* !!!
*/
func expression() (string, string) {
	// log.Print("expression")
	atyp := ""
	aval := em.EMPTY
	btyp := ""
	bval := em.EMPTY
	typ := ""
	for {
		btyp, bval = mulexpr()
		// log.Print("from addexpr ", atyp, btyp)
		if atyp == "" {
			atyp = btyp
			aval = bval
		} else if atyp != btyp {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Mismatch add-expression %v vs. %v", l, c, atyp, btyp)
			errcount++
		} else if atyp == "$NUMBER" {
			if typ != "$PLUS" && typ != "$MINUS" {
				l, c := token.GetLineCol()
				log.Printf("DAP.p %v:%v -- Illegal operators on numbers %v", l, c, typ)
				errcount++
			} else if aval == em.EMPTY && bval == em.EMPTY {
				em.GenOp2Cmd(typ, atyp, aval, btyp, bval) // +/-
				bval = em.EMPTY
			} else {
				anval, erra := strconv.Atoi(aval)
				bnval, errb := strconv.Atoi(bval)
				if erra != nil || errb != nil {
					em.GenOp2Cmd(typ, atyp, aval, btyp, bval) // +/-
					bval = em.EMPTY
				} else if typ == "$PLUS" {
					bval = strconv.Itoa(anval + bnval)
				} else { // $MINUS
					bval = strconv.Itoa(anval - bnval)
				}
			}
		} else if atyp == "$BOOL" {
			if typ == "$OR" {
				if aval != em.EMPTY && bval != em.EMPTY {
					if aval == em.TRUE || bval == em.TRUE {
						bval = em.TRUE
					} else {
						bval = em.FALSE
					}
				} else {
					bval = em.EMPTY
					em.GenOp2Cmd(typ, atyp, aval, btyp, bval) // OR
				}
			} else {
				l, c := token.GetLineCol()
				log.Printf("DAP.p %v:%v -- Illegal operator on boolean %v", l, c, typ)
				errcount++
			}
		} else if typ == "$CHARRAY" {
			if typ != "$PLUS" { // dunno what to generate yet
				l, c := token.GetLineCol()
				log.Printf("DAP.p %v:%v -- illegal operation %v on %v", l, c, typ, atyp)
				errcount++
			} else {
				// genCode?
				l, c := token.GetLineCol()
				log.Printf("DAP.p %v:%v -- Unknown string operation %v", l, c, typ)
				errcount++
			}
		}
		if typ = token.Peek(); typ != "$PLUS" && typ != "$MINUS" && typ != "$OR" {
			break
		}
		token.Next()
	}
	return btyp, bval
}

/* expr {, expr}*
 */
func expression_list(gen func(string, string)) int {
	// log.Print("expressions")
	count := 0
	for {
		etyp, eval := expression()
		count++
		gen(etyp, eval)
		if typ := token.Peek(); typ != "$COMMA" {
			break
		}
		token.Next()
	}
	return count
}

/* variable <- expr
 */
func assignment(lvl int) {
	token.Next()
	// log.Print("assignment ", token.Val)
	assgline := token.GetLine() // this assignment line number
	attr := varcoll[parent+":"+token.Val]
	vtyp := attr.typ
	if vtyp == "$INT" || vtyp == "$REAL" {
		vtyp = "$NUMBER"
	}
	// keep token, then...
	expect("$ASSG", "<- expected")
	etyp, eval := expression()
	if vtyp != etyp {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Type mismatch in assignment", l, c)
		errcount++
	}
	em.GenStore(etyp, attr.parent, attr.loc, eval)
	em.CollectAssg(attr.parent, assgline, attr.loc)
}

/* input variable_list
 */
func input_stmt(lvl int) {
	//log.Print("input stmt")
	token.Next()
	inpline := token.GetLine() // this input statement line number
	namelist := variable_list(em.GenInp)
	for _, v := range namelist {
		attr := varcoll[parent+":"+v]
		em.CollectAssg(parent, inpline, attr.loc)
	}
}

/* output expression_list
 */
func output_stmt(lvl int) {
	//log.Print("output stmt")
	token.Next()
	expression_list(em.GenOut)
}

/* while bool_expr do code_list endwhile
 */
func while_stmt(lvl int) {
	//log.Print("while stmt")
	expect("$WHILE", "while expected")
	l1 := em.GenLabel()
	l2 := em.GenLabel()
	em.GenLoc(l1)
	em.GenLine(token.GetLineCol()) //!
	etyp, eval := expression()
	if etyp != "$BOOL" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Non boolean condition", l, c)
		errcount++
	} else if eval == em.TRUE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Infinite loop!", l, c)
		errcount++
	} else if eval == em.FALSE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Loop never entered!", l, c)
		errcount++
	}
	em.GenCond(l2, eval)
	expect("$DO", "do expected") // or skip("$WHILE")
	code_block(lvl + 1)
	if skip("$ENDWHILE") { // optional
		em.GenLine(token.GetLineCol()) //!
	}
	em.GenGoto(l1)
	em.GenLoc(l2)
}

/* repeat code_list until bool_expr
 */
func repeat_stmt(lvl int) {
	// log.Print("repeat-until stmt")
	expect("$REPEAT", "repeat expected")
	l1 := em.GenLabel()
	em.GenLoc(l1)
	em.GenLine(token.GetLineCol()) //!
	code_block(lvl + 1)
	expect("$UNTIL", "until expected")
	em.GenLine(token.GetLineCol())
	etyp, eval := expression()
	if etyp != "$BOOL" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Non boolean condition", l, c)
		errcount++
	} else if eval == em.TRUE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Useless repeat-until", l, c)
		errcount++
	} else if eval == em.FALSE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p  %v:%v -- Infinite repeat-until loop", l, c)
		errcount++
	}
	em.GenCond(l1, eval)
}

/* if bool_expr then code_list
   {elif bool_expr then code_list}* [else code_list] endif
*/
func if_stmt(lvl int) {
	// log.Print("if-then-else stmt")
	expect("$IF", "if expected")
	etyp, eval := expression()
	if etyp != "$BOOL" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Non boolean condition", l, c)
		errcount++
	} else if eval == em.TRUE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Then part is always executed", l, c)
		errcount++
	} else if eval == em.FALSE {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Never entering the then part", l, c)
		errcount++
	}
	lels := em.GenLabel()
	lfin := em.GenLabel()
	em.GenCond(lels, eval)
	expect("$THEN", "then expected") // or skip("$THEN")
	code_block(lvl + 1)
	typ := token.Next()
	for typ == "$ELIF" { // elif blocks
		em.GenGoto(lfin)
		em.GenLoc(lels)
		lels = em.GenLabel()
		syncStartBlock(lvl)
		em.GenLine(token.GetLineCol()) //!
		etyp, eval = expression()
		if etyp != "$BOOL" {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Non boolean condition", l, c)
			errcount++
		} else if eval == em.TRUE {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Else if part is always executed", l, c)
			errcount++
		} else if eval == em.FALSE {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- This else if part is never entered", l, c)
			errcount++
		}
		em.GenCond(lels, eval)
		expect("$THEN", "then of elif expected") // or skip("$THEN")
		code_block(lvl + 1)
		typ = token.Next()
	}
	if typ == "$ELSE" { // else block
		em.GenGoto(lfin)
		em.GenLoc(lels)
		em.GenLine(token.GetLineCol()) //!
		syncStartBlock(lvl)
		code_block(lvl + 1)
	} else {
		em.GenLoc(lels)
	}
	em.GenLoc(lfin)
	em.GenLine(token.GetLineCol()) //!
	if skip("$ENDIF") {            // optional
		em.GenLine(token.GetLineCol()) //!
	}
	// log.Print("endif", token.Typ, token.Val, token.Cno)
}

/* case expr of {expr : code_list}* [otherwise code_list] endcase
   labels either "expr : ...", "expr ) ...", or "expr :) ..."
*/
func case_stmt(lvl int) {
	// log.Print("case stmt")
	expect("$CASE", "case expected")
	ctyp, cval := expression()
	if cval != em.EMPTY {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Useless switch/case, has constant expression", l, c)
		// errcount++
		em.GenConst(ctyp, cval)
	}
	lfin := em.GenLabel()
	expect("$OF", "of expected")
	for isStartExpression() {
		em.GenLine(token.GetLineCol()) //!
		em.GenDup()                    // make a copy of case expression, vs. label expression
		ltyp, lval := expression()
		if ltyp != ctyp {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v.%v -- mismatch case label type", l, c)
			errcount++
		} else if lval == em.EMPTY {
			l, c := token.GetLineCol()
			log.Printf("DAP.p %v:%v -- Should have constant value as label", l, c)
			// errcount++
		}
		lnex := em.GenLabel()
		em.GenCase(lnex, ltyp, lval)

		skip("$COLON")
		skip("$RIGHTPAR")
		code_block(lvl + 1)
		em.GenGoto(lfin)
		em.GenLoc(lnex)
	}
	if token.Typ == "$DEFAULT" { // default block
		em.GenLine(token.GetLineCol()) //!
		skip("$DEFAULT")
		skip("$COLON")
		skip("$RIGHTPAR")
		code_block(lvl + 1)
	}
	em.GenLoc(lfin)
	em.GenPop()
	if skip("$ENDCASE") { // optional
		em.GenLine(token.GetLineCol()) //!
	}
}

var tab = 0

func syncStartBlock(lvl int) {
	// log.Print("starting new block ", token.Typ)
	if !token.First {
		for !token.First {
			token.Next()
		}
		token.PushBack()
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Must be at the beginning of line", l, c)
		errcount++
	} else if tab == 0 && lvl == 1 {
		tab = token.Cno
	} else if lvl*tab != token.Cno {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Inconsistent block indentation %v vs. %v", l, c, lvl*tab, token.Cno)
		errcount++
	}
}

func checkBlockLevel(sp, lvl int) {
	if tab == 0 && lvl == 1 {
		tab = sp
	} else if lvl*tab != sp {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Inconsistent block indentation %v vs. %v", l, c, lvl*tab, sp)
		errcount++
	}
}

// tab decreases, at least (can be more) by one tab
func upLevel(sp, lvl int) bool {
	return (lvl-1)*tab >= sp
}

/* code CR { code CR }*
 */
func code_block(lvl int) {
	// log.Print("block of codes ", lvl)
	for {
		typ := token.Peek()
		/*
						if !token.First {
							for !token.First {
								token.Next()
							}
							token.PushBack()
							log.Print("must be at the beginning of line")
			                errcount++
						} else {
							checkBlockLevel(token.Cno, lvl)
						}
		*/
		// log.Print("indent", token.Cno, lvl, token.Typ, token.Val)
		if typ == "$ENDPROG" || upLevel(token.Cno, lvl) {
			break
		}
		syncStartBlock(lvl)
		switch typ {
		case "$NAME":
			em.GenLine(token.GetLineCol())
			assignment(lvl)

		case "$INPUT":
			em.GenLine(token.GetLineCol())
			input_stmt(lvl)

		case "$OUTPUT":
			em.GenLine(token.GetLineCol())
			output_stmt(lvl)

		case "$WHILE":
			while_stmt(lvl)

		case "$REPEAT":
			repeat_stmt(lvl)

		case "$IF":
			em.GenLine(token.GetLineCol())
			if_stmt(lvl)

		case "$CASE":
			em.GenLine(token.GetLineCol())
			case_stmt(lvl)

		default: // remove the impending token
			token.Next()
		}
	}
}

/* block of codes
 */
func algorithm() {
	// log.Print("algorithm")
	code_block(1)
}

func programBlock() {
	sync("$PROGRAM", "$DICT", "$ENDPROG")
	if token.Next() != "$PROGRAM" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Missing program header, found %v/%v", l, c, token.Val, token.Typ)
		errcount++
	}
	em.GenLine(token.GetLineCol()) // trace starts from program line
	if token.Next() != "$NAME" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Missing program name, found %v/%v", l, c, token.Val, token.Typ)
		errcount++
	} else {
		// log.Println("Parsing", token.Val)
	}
	// sync("$DICT", "$CODE", "$ENDPROG")
	if typ := token.Next(); typ != "$DICT" && typ != "$GLOBAL" && typ != "$LOCAL" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Missing data section, found %v/%v", l, c, token.Val, token.Typ)
		errcount++
	}
	declaration()
	sync("$CODE", "$NAME", "$ENDPROG")
	if token.Next() != "$CODE" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Missing code section, found %v/%v", l, c, token.Val, token.Typ)
		errcount++
	}
	em.GenLine(token.GetLineCol()) //!
	algorithm()
	em.GenLine(token.GetLineCol())
	em.GenExit()
	sync("$ENDPROG", "$ENDPROG", "$ENDPROG")
	if token.Next() != "$ENDPROG" {
		l, c := token.GetLineCol()
		log.Printf("DAP.p %v:%v -- Missing end program, found %v/%v", l, c, token.Val, token.Typ)
		errcount++
	} else {
		// log.Println("Program ends")
	}
}

func endParse() {
	/*
		for token.IsAvail() {
			log.Printf("%v:%v: %v %v\n", token.Lno, token.Cno, token.Typ, token.Val)
		}
	*/
	if err := token.Err(); err != nil {
		log.Fatalf("DAP.p *** Fatal error %v", err)
	}
	toterr := token.ErrCount() + errcount
	if toterr > 0 {
		log.Fatalf("*** DAP compilation error count %v", toterr)
	} else {
		log.Printf("*** DAP compilation successful")
	}
}

func Compile(t *scanner.Token) {
	log.Printf("*** DAP compiling")
	token = t
	programBlock()
	endParse()
}

func ProcessSymbols() {
	for vname, vattr := range varcoll {
		em.CollectVariable(vattr.parent, strings.TrimPrefix(vname, vattr.parent)[1:], vattr.typ, vattr.val, vattr.loc)
	}
}
