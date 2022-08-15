package emulator

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

const (
	NOP    = 0   // no operation
	CMT    = 1   // comment in symbolic, nop in numeric form
	LINE   = 2   // line and col in source
	GVAR   = 3   // var, global variable
	LVAR   = 4   // func var, local variable
	CLAIM  = 11  // spc=stack[TOP]; stack[TOP]=BASE; BASE=TOP; TOP+=spc
	FREE   = 12  // TOP=BASE; BASE=stack[TOP]; TOP-- // if stack[TOP] < BASE
	COPY   = 13  // stack[TOP]=stack[stack[TOP]] // global var
	STORE  = 14  // stack[stack[TOP]]=stack[TOP-1]; TOP-=2
	LCOPY  = 15  // stack[TOP]=stack[BASE+stack[TOP]] // local var, rename
	LSTOR  = 16  // stack[BASE+stack[TOP]]=stack[TOP-1]; TOP-=2 // rename
	PUSH   = 21  // TOP++; stack[TOP]=CODE[IP]; IP++
	POP    = 22  // TOP--
	SWAP   = 23  // stack[TOP] <-> stack[TOP-1]
	DUP    = 24  // TOP++; stack[TOP]=stack[TOP-1] // new change
	NEG    = 41  // stack[TOP] = -stack[TOP]
	ADD    = 42  // stack[TOP-1] = stack[TOP-1] + stack[TOP]; TOP--
	SUB    = 43  // stack[TOP-1] = stack[TOP-1] - stack[TOP]; TOP--
	MUL    = 44  // stack[TOP-1] = stack[TOP-1] * stack[TOP]; TOP--
	DIV    = 45  // stack[TOP-1] = stack[TOP-1] / stack[TOP]; TOP--
	MOD    = 46  // stack[TOP-1] = stack[TOP-1] % stack[TOP]; TOP--
	NOT    = 51  // stack[TOP] = !stack[TOP]
	OR     = 52  // stack[TOP-1] = stack[TOP-1] || stack[TOP]; TOP--
	AND    = 53  // stack[TOP-1] = stack[TOP-1] && stack[TOP]; TOP--
	LT     = 61  // stack[TOP-1] = stack[TOP-1] < stack[TOP]; TOP--
	LEQ    = 62  // stack[TOP-1] = stack[TOP-1] <= stack[TOP]; TOP--
	GT     = 63  // stack[TOP-1] = stack[TOP-1] > stack[TOP]; TOP--
	GEQ    = 64  // stack[TOP-1] = stack[TOP-1] >= stack[TOP]; TOP--
	EQ     = 65  // stack[TOP-1] = stack[TOP-1] == stack[TOP]; TOP--
	NEQ    = 66  // stack[TOP-1] = stack[TOP-1] != stack[TOP]; TOP--
	INPI   = 71  // push(getint())
	INPC   = 72  // push(getchar())
	INPB   = 73  // push(getbool())
	OUTI   = 81  // putint(pop())
	OUTC   = 82  // putchar(pop())
	OUTB   = 83  // putbool(pop())
	SAVEIP = 201 // push(IP)
	COND   = 202 // if stack[TOP-1] then IP=stack[TOP]; TOP -= 2
	NCOND  = 203 // if !stack[TOP-1] then IP=stack[TOP]; TOP -= 2
	CALL   = 204 // stack[TOP] <-> IP // call addr
	GOTO   = 205 // IP=stack[TOP]; TOP-- // also RETURN
	EXIT   = 255
)

type codes []int
type memory []int
type tags []byte
type anime struct {
	cmd byte
	val int
}

const memSIZE = 9999

type tagVal struct {
	C byte
	V interface{}
}

var tf = map[bool]int{false: 0, true: 1}

var (
	stack    memory
	prog     codes
	errcount = 0
	iR       = NOP
	iP       = 0
	base     = 0
	top      = -1
	maxtop   = 0
	step     = 0
	done     = false
	strinput = ""
)

const (
	traceInitial = iota
	traceReady
	traceMore
	traceInfinite
	traceError
	traceInput
	traceExit
)

/* command tags:
L ask for source program and variables offset (at the start of animation)
I deliver input from web user
C do continue with antoher batch of steps
X web user also agree to terminate (usually when web user receives an E signal
R please reset the process to start from the beginning again

   log tags:
C inform web user a batch of steps has been reached
L executing source code at particular line
X illegal operation (usually when using uninitialized variable)
V informing web user on a new value of a variable at the executed source line
I asking input from the web user (which replied also by I command tag)
O sending output value to the web user
E inform web user the program has reached a normal termination
P sending source program
D sending variable offsets
A sending (linenum,offset) pairs
*/
func webResponder(srcFile string, traceStatus int, chlog chan<- []byte, chcmd <-chan []byte) int {
	var respond tagVal
	respJson := <-chcmd
	json.Unmarshal(respJson, &respond)
	// log.Print(string(respJson))
	// log.Printf("%c:%v", respond.C, respond.V)
	switch respond.C {
	case 'L': // ask for source program and variables' offsets
		if sf, err := os.Open(srcFile); err != nil {
			log.Print(err)
		} else if srcProg, err := ioutil.ReadAll(sf); err != nil {
			log.Print(err)
		} else {
			sf.Close()
			sources := []tagVal{tagVal{'P', string(srcProg)}, tagVal{'D', varcoll}, tagVal{'A', asscoll}}
			if srcJson, err := json.Marshal(sources); err != nil {
				log.Print(err)
			} else {
				chlog <- srcJson
			}
			// log.Print(string(srcProg))
		}
		traceStatus = traceReady

	case 'I': // delivering input from user
		// if traceStatus == traceInput {}
		top++
		switch iR {
		case INPI:
			fmt.Sscan(respond.V.(string), &stack[top])
			strinput = ""
			// log.Print("input I = ", stack[top])
		case INPB:
			var b bool
			fmt.Sscan(respond.V.(string), &b)
			stack[top] = tf[b]
			strinput = ""
			// log.Print("input B = ", b)
		case INPC:
			/*
				if strinput, err := strconv.Unquote("`" + respond.V.(string) + "`"); err != nil {
					log.Print(err)
					strinput = ""
				} else {
					stack[top] = int(strinput[0])
					strinput = strinput[1:]
				}
			*/
			strinput = respond.V.(string)
			stack[top] = int(strinput[0])
			strinput = strinput[1:]
			// log.Printf("input C = %c", stack[top])
		}
		traceStatus = traceMore

	case 'C': // Continue with another next allocated steps
		// if traceStatus == traceInfinite {}
		step = 0
		traceStatus = traceMore

	case 'X': // Web user also agree to terminate
		done = true
		traceStatus = traceMore // ???
		//close(chcmd)

	case 'R': // Reset the process from the beginning
		iP = 0
		top = -1
		base = 0
		step = 0
		strinput = ""
		traceStatus = traceMore

	default:
		log.Print("Emu unknown user command")
		traceStatus = traceError
	}
	return traceStatus
}

// emulate with web front-end
func Wemulate(srcFile string, steps int, chint chan<- os.Signal, chlog chan<- []byte, chcmd <-chan []byte) {
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	stack = make(memory, memSIZE)
	empty := make([]bool, memSIZE)
	lastline := 0
	lastcol := 0
	trace := []tagVal{}
	base = 0
	top = -1
	iP = 0
	step = 0
	done = false
	/*
		    for prog[iP] != EXIT && step <= steps {
				// log.Print( "[",iP,"]", prog[iP], prog[iP+1],"|",top,":", stack[:10] )
			    fmt.Println(trace)
	*/
	traceStatus := traceInitial
	for {
		if maxtop < top {
			maxtop = top
		}
		/*
			1. if have enough trace (aka looping)/
			      need input/
			      hit an error/
			      execution terminates,
			2. send trace to browser
			3. then wait for browser responds
			      expecting: input, continue, exit, restart
			4. otherwise check if browser sends some command
			      expecting: exit, restart
			      and/or continue with the next instruction execution
		*/
		for traceStatus != traceMore {
			if len(trace) == 0 { // dont send nothing
			} else if traceJson, err := json.Marshal(trace); err != nil {
				log.Printf("DAP.e %v:%v -- Fail to marshal trace data", lastline, lastcol)
			} else {
				// log.Print("Emu service sending traces")
				chlog <- traceJson
				// log.Print("Emu service done sending")
			}
			traceStatus = webResponder(srcFile, traceStatus, chlog, chcmd)
			if traceStatus == traceError {
				trace = []tagVal{tagVal{'X', "Unknown user respond, please repeat"}}
			} else {
				trace = []tagVal{}
			}
		}
		if done {
			break
		}
		step++
		if step > steps {
			trace = append(trace, tagVal{C: 'C'})
			traceStatus = traceInfinite
		} else {
			switch iR, iP = prog[iP], iP+1; iR {
			case NOP, CMT:
			case LINE:
				lastline = prog[iP]
				lastcol = prog[iP+1]
				trace = append(trace, tagVal{'L', lastline})
				iP += 2
			case GVAR:
				iP++
			case LVAR:
				iP += 2
			case CLAIM:
				spc := stack[top]
				stack[top] = base
				base = top
				top += spc
				for si := base + 1; si <= top; si++ {
					stack[si] = rnd.Intn(7919) + 2
					empty[si] = true
				}
			case FREE:
				top = base
				base = stack[top]
				top--
			case COPY:
				if empty[top] {
					trace = append(trace, tagVal{'X', "Illegal access to uninitialized variable"})
					traceStatus = traceError
					log.Printf("DAP.e %v:%v -- Illegal access to uninitialized variable, %v", lastline, lastcol, stack[top])
					errcount++
				}
				stack[top] = stack[stack[top]]
			case STORE:
				trace = append(trace, tagVal{'V', stack[top-1]})
				stack[stack[top]] = stack[top-1]
				empty[stack[top]] = false
				top -= 2
			case LCOPY:
				if empty[top] {
					trace = append(trace, tagVal{'X', "Illegal access to uninitialized local variable"})
					traceStatus = traceError
					log.Printf("DAP.e %v:%v -- Illegal access to uninitialized local variable, %v", lastline, lastcol, stack[top])
					errcount++
				}
				stack[top] = stack[base+stack[top]]
			case LSTOR:
				trace = append(trace, tagVal{'V', stack[top-1]})
				stack[base+stack[top]] = stack[top-1]
				empty[base+stack[top]] = false
				top -= 2
			case PUSH:
				top++
				stack[top] = prog[iP]
				iP++
			case POP:
				top--
			case DUP:
				top++
				stack[top] = stack[top-1]
			case SWAP:
				stack[top], stack[top-1] = stack[top-1], stack[top]
			case NEG:
				stack[top] = -stack[top]
			case ADD:
				stack[top-1] = stack[top-1] + stack[top]
				top--
			case SUB:
				stack[top-1] = stack[top-1] - stack[top]
				top--
			case MUL:
				stack[top-1] = stack[top-1] * stack[top]
				top--
			case DIV:
				if stack[top] == 0 {
					trace = append(trace, tagVal{'X', "Illegal division by zero"})
					traceStatus = traceError
					log.Printf("DAP.e %v:%v -- Illegal division by zero", lastline, lastcol)
					errcount++

				} else {
					stack[top-1] = stack[top-1] / stack[top]
				}
				top--
			case MOD:
				if stack[top] == 0 {
					trace = append(trace, tagVal{'X', "Illegal modulo division by zero"})
					traceStatus = traceError
					log.Printf("DAP.e %v:%v -- Illegal modulo division by zero", lastline, lastcol)
					errcount++

				} else {
					stack[top-1] = stack[top-1] % stack[top]
				}
				top--
			case NOT:
				stack[top] = tf[stack[top] == 0]
			case OR:
				stack[top-1] = tf[stack[top-1] != 0 || stack[top] != 0]
				top--
			case AND:
				stack[top-1] = tf[stack[top-1] != 0 && stack[top] != 0]
				top--
			case LT:
				stack[top-1] = tf[stack[top-1] < stack[top]]
				top--
			case LEQ:
				stack[top-1] = tf[stack[top-1] <= stack[top]]
				top--
			case GT:
				stack[top-1] = tf[stack[top-1] > stack[top]]
				top--
			case GEQ:
				stack[top-1] = tf[stack[top-1] >= stack[top]]
				top--
			case EQ:
				stack[top-1] = tf[stack[top-1] == stack[top]]
				top--
			case NEQ:
				stack[top-1] = tf[stack[top-1] != stack[top]]
				top--
			case INPI:
				trace = append(trace, tagVal{C: 'I'})
				traceStatus = traceInput
			case INPC:
				if strinput == "" {
					trace = append(trace, tagVal{C: 'I'})
					traceStatus = traceInput
				} else {
					top++
					stack[top] = int(strinput[0])
					strinput = strinput[1:]
				}
			case INPB:
				trace = append(trace, tagVal{C: 'I'})
				traceStatus = traceInput
			case OUTI:
				trace = append(trace, tagVal{'O', stack[top]})
				top--
			case OUTC:
				trace = append(trace, tagVal{'O', fmt.Sprintf("%c", stack[top])})
				// trace = append(trace, tagVal{'O', strconv.QuoteRuneToASCII(rune(stack[top]))})
				// log.Print("output C ", strconv.QuoteRuneToASCII(rune(stack[top])))
				top--
			case OUTB:
				trace = append(trace, tagVal{'O', stack[top] != 0})
				top--
			case SAVEIP:
				top++
				stack[top] = iP
			case COND:
				if stack[top-1] != 0 {
					iP = stack[top]
				}
				top -= 2
			case NCOND:
				if stack[top-1] == 0 {
					iP = stack[top]
				}
				top -= 2
			case CALL:
				stack[top], iP = iP, stack[top]
			case GOTO:
				iP = stack[top]
				top--
			case EXIT:
				trace = append(trace, tagVal{C: 'E'})
				traceStatus = traceExit
				iP = 0
			}
		}
	}
	log.Printf("DAP.e *** Stopped after %v steps", step)
	close(chint)
}

func Emulate(steps int) {
	log.Print("*** DAP executing the codes")
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	base = 0
	top = -1
	iP = 0
	stack = make(memory, memSIZE)
	empty := make([]bool, memSIZE)
	step := 0
	lastline := 0
	lastcol := 0
	charline := false
	for prog[iP] != EXIT && step <= steps {
		if maxtop < top {
			maxtop = top
		}
		// log.Print( "[",iP,"]", prog[iP], prog[iP+1],"|",top,":", stack[:10] )
		step++
		switch iR, iP = prog[iP], iP+1; iR {
		case NOP, CMT:
		case LINE:
			lastline = prog[iP]
			lastcol = prog[iP+1]
			iP += 2
		case GVAR:
			iP++
		case LVAR:
			iP += 2
		case CLAIM:
			spc := stack[top]
			stack[top] = base
			base = top
			top += spc
			for si := base + 1; si <= top; si++ {
				stack[si] = rnd.Intn(7919) + 2
				empty[si] = true
			}
		case FREE:
			top = base
			base = stack[top]
			top--
		case COPY:
			if empty[top] {
				log.Printf("DAP.e %v:%v -- Illegal access to uninitialized variable, %v", lastline, lastcol, stack[top])
				errcount++
			}
			stack[top] = stack[stack[top]]
		case STORE:
			stack[stack[top]] = stack[top-1]
			empty[stack[top]] = false
			top -= 2
		case LCOPY:
			if empty[top] {
				log.Printf("DAP.e %v:%v -- Illegal access to uninitialized local variable, %v", lastline, lastcol, stack[top])
				errcount++
			}
			stack[top] = stack[base+stack[top]]
		case LSTOR:
			stack[base+stack[top]] = stack[top-1]
			empty[base+stack[top]] = false
			top -= 2
		case PUSH:
			top++
			stack[top] = prog[iP]
			iP++
		case POP:
			top--
		case DUP:
			top++
			stack[top] = stack[top-1]
		case SWAP:
			stack[top], stack[top-1] = stack[top-1], stack[top]
		case NEG:
			stack[top] = -stack[top]
		case ADD:
			stack[top-1] = stack[top-1] + stack[top]
			top--
		case SUB:
			stack[top-1] = stack[top-1] - stack[top]
			top--
		case MUL:
			stack[top-1] = stack[top-1] * stack[top]
			top--
		case DIV:
			if empty[top] {
				log.Printf("DAP.e %v:%v -- Illegal division by zero", lastline, lastcol)
				errcount++
			} else {
				stack[top-1] = stack[top-1] / stack[top]
			}
			top--
		case MOD:
			if empty[top] {
				log.Printf("DAP.e %v:%v -- Illegal modulo division by zero", lastline, lastcol)
			} else {
				stack[top-1] = stack[top-1] % stack[top]
			}
			top--
		case NOT:
			stack[top] = tf[stack[top] == 0]
		case OR:
			stack[top-1] = tf[stack[top-1] != 0 || stack[top] != 0]
			top--
		case AND:
			stack[top-1] = tf[stack[top-1] != 0 && stack[top] != 0]
			top--
		case LT:
			stack[top-1] = tf[stack[top-1] < stack[top]]
			top--
		case LEQ:
			stack[top-1] = tf[stack[top-1] <= stack[top]]
			top--
		case GT:
			stack[top-1] = tf[stack[top-1] > stack[top]]
			top--
		case GEQ:
			stack[top-1] = tf[stack[top-1] >= stack[top]]
			top--
		case EQ:
			stack[top-1] = tf[stack[top-1] == stack[top]]
			top--
		case NEQ:
			stack[top-1] = tf[stack[top-1] != stack[top]]
			top--
		case INPI:
			top++
			fmt.Scan(&stack[top])
		case INPC:
			top++
			fmt.Scanf("%c", &stack[top])
		case INPB:
			var b bool
			fmt.Scan(&b)
			top++
			stack[top] = tf[b]
		case OUTI:
			if charline {
				charline = false
				fmt.Println()
			}
			fmt.Println(stack[top])
			top--
		case OUTC:
			fmt.Printf("%c", stack[top])
			charline = true
			top--
		case OUTB:
			if charline {
				charline = false
				fmt.Println()
			}
			fmt.Println(stack[top] != 0)
			top--
		case SAVEIP:
			top++
			stack[top] = iP
		case COND:
			if stack[top-1] != 0 {
				iP = stack[top]
			}
			top -= 2
		case NCOND:
			if stack[top-1] == 0 {
				iP = stack[top]
			}
			top -= 2
		case CALL:
			stack[top], iP = iP, stack[top]
		case GOTO:
			iP = stack[top]
			top--
		}
	}
	log.Printf("DAP.e *** Stopped after %v steps, mem=%v", step, maxtop)
	if step > steps {
		log.Print("DAP.e -- Rerun using -time for more steps")
	}
}

var sym2num = map[string]int{
	"NOP":    0,
	"CMT":    1,
	"LINE":   2,
	"GVAR":   3,
	"LVAR":   4,
	"CLAIM":  11,
	"FREE":   12,
	"COPY":   13,
	"STORE":  14,
	"LCOPY":  15,
	"LSTOR":  16,
	"PUSH":   21,
	"POP":    22,
	"SWAP":   23,
	"DUP":    24,
	"NEG":    41,
	"ADD":    42,
	"SUB":    43,
	"MUL":    44,
	"DIV":    45,
	"MOD":    46,
	"NOT":    51,
	"OR":     52,
	"AND":    53,
	"LT":     61,
	"LEQ":    62,
	"GT":     63,
	"GEQ":    64,
	"EQ":     65,
	"NEQ":    66,
	"INPI":   71,
	"INPC":   72,
	"INPB":   73,
	"OUTI":   81,
	"OUTC":   82,
	"OUTB":   83,
	"SAVEIP": 201,
	"COND":   202,
	"NCOND":  203,
	"CALL":   204,
	"GOTO":   205,
	"EXIT":   255,
}

type labelrec struct {
	p int
	c []int
}

func asm(s4041 []string) {
	var ins, sp1, sp2 string
	var op1, op2 int
	prog = codes{}
	iP = 0
	labels := map[string]labelrec{}
	for _, cmd := range s4041 {
		fmt.Sscanln(cmd, &ins, &sp1, &sp2)
		op1, _ = strconv.Atoi(sp1)
		op2, _ = strconv.Atoi(sp2)
		// fmt.Println(iP, ins, sp1, sp2, cmd)
		iP++
		switch ins {
		case "LABEL":
			iP--
			if sp1[0] == '@' {
				l := labels[sp1]
				l.p = iP
				labels[sp1] = l
				// fmt.Println( "ref", sp1, labels[sp1])
			}
		case "LINE":
			// fmt.Print( " ", sym2num[ins], op1, op2 )
			prog = append(prog, sym2num[ins], op1, op2)
			iP += 2
		case "CMT":
			// fmt.Print( " ", sym2num["NOP"] )
			prog = append(prog, sym2num["NOP"])
		case "GVAR":
			// fmt.Print( " ", sym2num[ins], op1 )
			prog = append(prog, sym2num[ins], op1)
			iP++
		case "LVAR":
			// fmt.Print( " ", sym2num[ins], op1, op2 )
			prog = append(prog, sym2num[ins], op1, op2)
			iP += 2
		case "PUSH":
			// fmt.Print( " ", sym2num[ins], op1 )
			if sp1[0] == '@' {
				l := labels[sp1]
				l.c = append(l.c, iP)
				labels[sp1] = l
				// fmt.Println( "usg", sp1, labels[sp1])
			}
			prog = append(prog, sym2num[ins], op1)
			iP++
		default:
			// fmt.Print( " ", sym2num[ins] )
			prog = append(prog, sym2num[ins])
		}
	}
	for _, l := range labels {
		for _, op := range l.c {
			prog[op] = l.p
		}
	}
	// fmt.Println( prog )
}

var tok2sym = map[string]string{
	"$NOP":    "NOP",
	"$CMT":    "CMT",
	"$LINE":   "LINE",
	"$GVAR":   "GVAR",
	"$LVAR":   "LVAR",
	"$CLAIM":  "CLAIM",
	"$FREE":   "FREE",
	"$COPY":   "COPY",
	"$STORE":  "STORE",
	"$LCOPY":  "LCOPY",
	"$LSTOR":  "LSTOR",
	"$PUSH":   "PUSH",
	"$POP":    "POP",
	"$SWAP":   "SWAP",
	"$DUP":    "DUP",
	"$NEG":    "NEG",
	"$PLUS":   "ADD",
	"$MINUS":  "SUB",
	"$MULT":   "MUL",
	"$DIV":    "DIV",
	"$MOD":    "MOD",
	"$NOT":    "NOT",
	"$OR":     "OR",
	"$AND":    "AND",
	"$LT":     "LT",
	"$LEQ":    "LEQ",
	"$GT":     "GT",
	"$GEQ":    "GEQ",
	"$EQ":     "EQ",
	"$NEQ":    "NEQ",
	"$INPI":   "INPI",
	"$INPC":   "INPC",
	"$INPB":   "INPB",
	"$OUTI":   "OUTI",
	"$OUTC":   "OUTC",
	"$OUTB":   "OUTB",
	"$SAVEIP": "SAVEIP",
	"$COND":   "COND",
	"$NCOND":  "NCOND",
	"$CALL":   "CALL",
	"$GOTO":   "GOTO",
	"$EXIT":   "EXIT",
}

const EMPTY = "<EMPTY\x08\x08\x08\x08\x08NOT INITIALIZED>"
const TRUE = "true"
const FALSE = "false"

type scodes []string

var s4041 = scodes{}

func tv2nums(t, v string) string {
	newv := v
	if v != EMPTY {
		switch t {
		case "$NUMBER":
		case "$BOOL":
			newv = strconv.Itoa(tf[v == TRUE])
		case "$CHAR", "$CHARRAY":
			newv = strconv.Itoa(int(v[0]))
		}
	}
	return newv
}

func GenNil(t string, p string, loc int) {
	// log.Printf("NIL generated %v:%v", p, loc)
}

func GenClaim(spc int) {
	s4041 = append(s4041, "PUSH "+strconv.Itoa(spc), "CLAIM")
	// log.Print("CLAIM generated ", spc)
}

func GenFree() { // not yet, to be used for subprogram
}

func GenCopy(p string, loc int) {
	if p == "" {
		s4041 = append(s4041, "PUSH "+strconv.Itoa(loc), "COPY")
	} else {
		s4041 = append(s4041, "PUSH "+strconv.Itoa(loc), "LCOPY")
	}
	// log.Printf("Copy var %v:%v", p, loc)
}

func GenStore(t string, p string, loc int, v string) {
	if v != EMPTY {
		// log.Printf("Push constant first %v:%v", t, v)
		s4041 = append(s4041, "PUSH "+tv2nums(t, v))
	}
	s4041 = append(s4041, "PUSH "+strconv.Itoa(loc))
	if p == "" {
		s4041 = append(s4041, "STORE")
	} else {
		s4041 = append(s4041, "LSTOR")
	}
	// log.Printf("Store var %v:%v", p, loc)
}

func GenConst(t string, v string) {
	s4041 = append(s4041, "PUSH "+tv2nums(t, v))
	// log.Print("Push ", v)
}

func GenOpCmd(op string) {
	if tok2sym[op] == "" {
		log.Printf("DAP.e %v:%v -- Empty cmd %v", lastline, lastcol, op)
		errcount++
	}
	s4041 = append(s4041, tok2sym[op])
	// log.Print(op, " applied")
}

func GenOp2Cmd(op, atyp, aval, btyp, bval string) {
	if aval != EMPTY {
		s4041 = append(s4041, "PUSH "+tv2nums(atyp, aval))
	}
	if bval != EMPTY {
		s4041 = append(s4041, "PUSH "+tv2nums(btyp, bval))
	}
	if tok2sym[op] == "" {
		log.Printf("DAP.e %v:%v -- Empty cmd %v", lastline, lastcol, op)
		errcount++
	}
	s4041 = append(s4041, tok2sym[op])
	// log.Print(op, " applied")
}

func GenInp(t string, p string, loc int) {
	switch t {
	case "$NUMBER", "$INT", "$REAL":
		s4041 = append(s4041, "INPI")
	case "$BOOL":
		s4041 = append(s4041, "INPB")
	case "$CHAR", "$CHARRAY":
		s4041 = append(s4041, "INPC")
	}
	s4041 = append(s4041, "PUSH "+strconv.Itoa(loc))
	if p == "" {
		s4041 = append(s4041, "STORE")
	} else {
		s4041 = append(s4041, "PUD")
	}
	// log.Printf("INP generated %v:%v for %v", p, loc, t)
}

func GenOut(t string, v string) {
	if v != EMPTY {
		s4041 = append(s4041, "PUSH "+tv2nums(t, v))
	}
	switch t {
	case "$NUMBER", "$INT", "$REAL":
		s4041 = append(s4041, "OUTI")
	case "$BOOL":
		s4041 = append(s4041, "OUTB")
	case "$CHAR", "$CHARRAY":
		s4041 = append(s4041, "OUTC")
	}
	// log.Printf("OUT generated %v:%v", t, v)
}

var ilabel = 1000

func GenLabel() string {
	ilabel++
	return "@L" + strconv.Itoa(ilabel)
}

func GenLoc(l string) {
	s4041 = append(s4041, "LABEL "+l)
	// log.Printf("LABEL %v generated", l)
}

func GenGoto(l string) {
	s4041 = append(s4041, "PUSH "+l, "GOTO")
	// log.Printf("GOTO %v generated", l)
}

func GenCond(l, v string) {
	if v != EMPTY {
		s4041 = append(s4041, "PUSH "+tv2nums("$BOOL", v))
	}
	s4041 = append(s4041, "PUSH "+l, "NCOND")
	// log.Printf("COND %v generated", l)
}

func GenCase(l, t, v string) {
	if v != EMPTY {
		s4041 = append(s4041, "PUSH "+tv2nums(t, v))
	}
	s4041 = append(s4041, "NEQ", "PUSH "+l, "COND")
	// log.Printf("CASE %v generated", l)
}

func GenDup() {
	s4041 = append(s4041, "DUP")
	// log.Printf("DUP top stack")
}

func GenPop() {
	s4041 = append(s4041, "POP")
	// log.Printf("POP top stack")
}

var lastline = 0
var lastcol = 0

func GenLine(l, c int) {
	if l > lastline {
		lastline = l
		lastcol = c
		s4041 = append(s4041, "LINE "+strconv.Itoa(l)+" "+strconv.Itoa(c))
	}
	// log.Printf("LINE %v:%v", l, c)
}

func GenExit() {
	s4041 = append(s4041, "EXIT")
	// log.Print("EXIT done finished")
}

func GenInit() scodes {
	return s4041
}

func GenPrint() {
	fmt.Printf("len=%v\n", len(s4041))
	for i, v := range s4041 {
		fmt.Printf("%v %v\n", i, v)
	}
}

func GenCodes() {
	log.Print("*** DAP assembling the codes")
	asm(s4041)
}

func SaveSymbols(fname string) { // save assembly codes
	log.Printf("Saving symbolic %v", fname)
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panic(err)
	}
	for _, s := range s4041 {
		fmt.Fprintln(file, s)
	}
	file.Close()
}

func LoadSymbols(fname string) { // load assembly codes
	log.Printf("Loading symbolic %v", fname)
	file, err := os.Open(fname)
	if err != nil {
		log.Panic(err)
	}
	symload := bufio.NewScanner(file)
	for symload.Scan() {
		s4041 = append(s4041, symload.Text())
	}
	file.Close()
}

func SaveCodes(fname string) { // save machine codes
	log.Printf("Saving codes %v", fname)
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Panic(err)
	}
	for _, c := range prog {
		fmt.Fprint(file, c, " ")
	}
	file.Close()
}

func LoadCodes(fname string) { // load machine codes
	log.Printf("Loading codes %v", fname)
	file, err := os.Open(fname)
	if err != nil {
		log.Panic(err)
	}
	for {
		var v int
		_, err := fmt.Fscan(file, &v)
		if err != nil {
			break
		}
		prog = append(prog, v)
	}
	file.Close()
}

type (
	nameattr struct {
		Parent string
		Name   string
		Typ    string
		Val    string
		Loc    int
	}

	assgattr struct {
		Parent string
		Line   int
		Off    int
	}
)

var (
	varcoll = []nameattr{}
	asscoll = []assgattr{}
)

func CollectVariable(parent, name, typ, val string, loc int) { // collect variables attributes
	varcoll = append(varcoll, nameattr{parent, name, typ, val, loc})
}

func CollectAssg(parent string, line int, offset int) { // collect (line,func:variable) assignment
	asscoll = append(asscoll, assgattr{parent, line, offset})
}

func SaveVariables(fname string) { // save symbol tables
	log.Printf("Saving symbol tables %v", fname)
	symbols := []tagVal{tagVal{'D', varcoll}, tagVal{'A', asscoll}}
	if symJson, err := json.Marshal(symbols); err != nil {
		log.Print(err)
	} else if err := ioutil.WriteFile(fname, symJson, 0644); err != nil {
		log.Panic(err)
	}
}

func LoadVariables(fname string) { // load symbol tables
	log.Printf("Loading symbol tables %v", fname)

	if symJson, err := ioutil.ReadFile(fname); err != nil {
		log.Panic(err)
	} else {
		var symbols []tagVal
		json.Unmarshal(symJson, &symbols)
		for _, v := range symbols {
			switch attr := v.V.(type) {
			case []nameattr:
				varcoll = attr
			case []assgattr:
				asscoll = attr
			}
		}
	}
}
