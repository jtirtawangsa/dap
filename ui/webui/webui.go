package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"honnef.co/go/js/dom"
)

type (
	tagValue struct {
		C byte
		V interface{}
	}

	signalChannel chan *int

	nameattr struct {
		Parent string
		Name   string
		Typ    string
		Val    string
		Loc    int
	}

/*
	assgattr struct {
		Parent string
		Line   int
		Off    int
	}
*/
)

var (
	stepByStep    = true
	traceEachLine = true
	askForInput   = false
	askForMore    = false
	runAnimation  = true
	chCmd         chan tagValue
	chStep        signalChannel
	chLog         signalChannel
	trace         []tagValue
	symbols       map[string]nameattr
	line2off      map[int]string
	linecount     = 0
	// line2off      map[int]assgattr
	d            = dom.GetWindow().Document()
	area_console dom.Element
	area_input   dom.Element
	area_output  dom.Element
	area_program dom.Element
	area_memory  dom.Element
	msg_field    *dom.HTMLInputElement
	butt_step    *dom.HTMLButtonElement
	butt_trace   *dom.HTMLButtonElement
	butt_run     *dom.HTMLButtonElement
	butt_restart *dom.HTMLButtonElement
	butt_exit    *dom.HTMLButtonElement
)

func doAnimation() {
	lastline := 0
	lastarg := 0
	chCmd <- tagValue{C: 'L'}
	<-chLog // wait until the source is loaded
	for _, v := range trace {
		switch v.C {
		case 'P': // source program, replace eol by <br>
			// store in program area
			/*
				f := func(c rune) bool {
					return c == '\r' || c == '\n'
				}
				linesPrg := strings.FieldsFunc(v.V.(string), f)
			*/
			linesPrg := strings.Split(v.V.(string), "\n")
			srcPrg := ""
			for i, l := range linesPrg {
				srcPrg += "<pre style='margin:0px;' id=L:" + strconv.Itoa(i+1) + "> " + l + "</pre>"
			}
			area_program.SetInnerHTML(srcPrg)
			// log.Print(srcPrg)

		case 'D': // symbol table, store internally
			// along with 'A' to aid the 'V' memory area
			// log.Print(v.V)
			symbols = make(map[string]nameattr)
			vars := ""
			for _, vsym := range v.V.([]interface{}) {
				vsym2 := vsym.(map[string]interface{})
				parent := vsym2["Parent"].(string)
				name := vsym2["Name"].(string)
				typ := vsym2["Typ"].(string)
				val := vsym2["Val"].(string)
				loc := int(vsym2["Loc"].(float64))
				key := parent + "-" + strconv.Itoa(loc)
				symbols[key] = nameattr{parent, name, typ, val, loc}
				if val == "<EMPTY\x08\x08\x08\x08\x08NOT INITIALIZED>" {
					vars += "<pre id=V-" + key + ">" + name + "=-NOT-INITIALIZED-</pre>"
				} else {
					vars += "<pre id=V-" + key + ">" + name + "=" + val + "</pre>"
				}
			}
			area_memory.SetInnerHTML(vars)
			// log.Print(area_memory.OuterHTML())

		case 'A': // (line,offset) pairs, store internally
			// along with 'D', to aid the 'V' memory area
			// log.Print(v.V)
			// line2off = make(map[int]assgattr)
			line2off = make(map[int]string)
			for _, vloc := range v.V.([]interface{}) {
				vloc2 := vloc.(map[string]interface{})
				parent := vloc2["Parent"].(string)
				line := int(vloc2["Line"].(float64))
				off := int(vloc2["Off"].(float64))
				// line2off[line] = assgattr{parent, line, off}
				if rec, ok := line2off[line]; ok { // already exists, multiple entry as in input parameters
					line2off[line] = rec + ":" + parent + "-" + strconv.Itoa(off)
				} else {
					line2off[line] = parent + "-" + strconv.Itoa(off)
				}
				// sym := symbols[line2off[line]]
				// log.Print(line, sym.Name, "=", sym.Val)
			}
			// log.Print(line2off)
		}
	}
	lastsrc := d.GetElementByID("L:1").(*dom.HTMLPreElement)
	last_varea := d.GetElementByID("V--1").(*dom.HTMLPreElement)
	chCmd <- tagValue{C: 'R'}
	msg_field.Value = "Reloaded, press a button"
	for runAnimation {
		<-chLog // wait until the next trace is available
		msg_field.Value = ""
		for _, v := range trace {
			if stepByStep { // wait for a signal, kinda sync.Cond
				<-chStep
			} else { // drain the channel
				select {
				case <-chStep:
				default:
				}
			}
			switch v.C {
			case 'E': // execution finished, show in the message area,
				// remind which buttons are useful, X or R in particular
				// msg: Button selection reminder
				msg_field.Value = "Program terminated. (eXit, or Restart)?"

			case 'C': // step too many, show in the message area,
				// remind which buttons are useful, C or X in particular
				// msg: Button selection reminder
				msg_field.Value = "Too many steps. (Step, Trace, Run, eXit, or Restart)?"
				askForMore = true

			case 'X': // show in message area, remind with buttons are useful
				// msg: from trace, and button selection
				msg_field.Value = "Error: " + v.V.(string)

			case 'L': // animate source program in program area
				//  keep the linenum in case variable changes
				if traceEachLine {
					linecount++
					lastline = int(v.V.(float64))
					key := "L:" + strconv.Itoa(lastline)
					srcline := d.GetElementByID(key).(*dom.HTMLPreElement)
					if lastsrc != srcline {
						lastsrc.Style().SetProperty("background-color", "white", "")
						srcline.Style().SetProperty("background-color", "yellow", "")
						lastsrc = srcline
						if !stepByStep {
							msg_field.Value = "Step: " + strconv.Itoa(linecount) + " line " + strconv.Itoa(lastline)
							time.Sleep(time.Second)
						}
					}
				}

			case 'V': // change of variables, put in memory area
				if traceEachLine {
					val := "<nil>"
					switch v := v.V.(type) { // all of them will be float64!
					case int:
						val = strconv.Itoa(v)
					case float64:
						val = strconv.FormatFloat(v, 'G', -1, 64)
					case bool:
						val = strconv.FormatBool(v)
					case string:
						val = v
					}
					memtext := area_memory.InnerHTML()
					if memtext != "" {
						memtext += "<br>"
					}
					if off, ok := line2off[lastline]; !ok {
						area_memory.SetInnerHTML(memtext + strconv.Itoa(lastline) + " ->" + val)
					} else {
						// chop off values, based on ":"
						offs := strings.Split(off, ":")
						if lastarg >= len(offs) {
							lastarg = 0
						}
						if sym, ok := symbols[offs[lastarg]]; !ok {
							area_memory.SetInnerHTML(memtext + off + " ->" + val)
						} else {
							// log.Print(off, sym.Name, sym.Val)
							key := "V-" + sym.Parent + "-" + strconv.Itoa(sym.Loc)
							var_area := d.GetElementByID(key).(*dom.HTMLPreElement)
							if last_varea != var_area {
								last_varea.Style().SetProperty("background-color", "white", "")
								last_varea = var_area
							}
							var_area.Style().SetProperty("background-color", "yellow", "")
							switch sym.Typ {
							case "$BOOL":
								if val == "1" {
									val = "true"
								} else {
									val = "false"
								}
							case "$CHAR", "$CHARRAY":
								val = strconv.QuoteRuneToASCII(rune(v.V.(float64)))
							}
							var_area.SetTextContent(sym.Name + "=" + val)
							// area_memory.SetInnerHTML(memtext + sym.Parent + ":" + sym.Name + "=" + sym.Val + " ->" + val)
							sym.Val = val
							symbols[off] = sym
							if !stepByStep {
								time.Sleep(time.Second)
							}
							// log.Print(off, sym.Name, sym.Val)
						}
						if len(offs) > 1 {
							lastarg = lastarg + 1
						} else {
							lastarg = 0
						}

					}
				}

			case 'I': // enable input field, remind the user in message area
				// the input is shown in console and input area by doCommands
				// enable input field, msg: "Enter input"
				msg_field.Value = "<<= Enter a value for input"
				askForInput = true

			case 'O': // put output in console and output area
				val := "<nil>"
				br := "<br>"
				switch v := v.V.(type) {
				case int:
					val = strconv.Itoa(v)
				case float64:
					val = strconv.FormatFloat(v, 'F', -1, 64)
				case bool:
					val = strconv.FormatBool(v)
				case string:
					val = v
					if val[0] != '\n' {
						br = ""
					}
				}
				context := area_console.InnerHTML()
				outtext := area_output.InnerHTML()
				if context != "" {
					context += br
				}
				if outtext != "" {
					outtext += br
				}
				area_console.SetInnerHTML(context + val)
				area_output.SetInnerHTML(outtext + val)
			}
		}
		// chCmd <- tagValue{C: 'C'}
	}
}

func callServer() {
	for runAnimation { // until 'X'
		cmd := <-chCmd
		switch cmd.C {
		case 'I': // input
			// log.Print("received ", string(cmd.C), " ", cmd.V)
			text := area_console.InnerHTML()
			if text != "" {
				text += "<br>"
			}
			area_console.SetInnerHTML(text + "<b><em>" + cmd.V.(string) + "</em></b>")
			text = area_input.InnerHTML()
			if text != "" {
				text += "<br>"
			}
			area_input.SetInnerHTML(text + "<b><em>" + cmd.V.(string) + "</em></b>")

		case 'R': // restart
			area_console.SetInnerHTML("")
			area_output.SetInnerHTML("")
			area_input.SetInnerHTML("")
			msg_field.Value = "Ready, press a button"
			trace = []tagValue{}
			linecount = 0

		case 'X': // exit
			msg_field.Value = "Program terminated, restart server side animator"

		case 'C': // load more trace

		case 'L': // load program source and symbol table

		default:
			log.Print("received ", string(cmd.C))
		}

		if cmdJson, err := json.Marshal(cmd); err != nil {
			log.Print(err)
		} else if resp, err := http.Post("/daptrace", "application/json", strings.NewReader(string(cmdJson))); err != nil {
			log.Print(err)
		} else {
			defer resp.Body.Close()
			if traceJson, err := ioutil.ReadAll(resp.Body); err != nil {
				log.Print(err)
			} else {
				json.Unmarshal(traceJson, &trace)
				chLog <- nil
				// log.Print(trace)
			}
		}
	}
}

func main() {
	area_console = d.GetElementByID("conarea")
	area_output = d.GetElementByID("outarea")
	area_input = d.GetElementByID("inparea")
	area_program = d.GetElementByID("prgarea")
	area_memory = d.GetElementByID("memarea")
	msg_field = d.GetElementByID("errdev").(*dom.HTMLInputElement)
	butt_step = d.GetElementByID("step").(*dom.HTMLButtonElement)
	butt_trace = d.GetElementByID("trace").(*dom.HTMLButtonElement)
	butt_run = d.GetElementByID("run").(*dom.HTMLButtonElement)
	butt_restart = d.GetElementByID("restart").(*dom.HTMLButtonElement)
	butt_exit = d.GetElementByID("exit").(*dom.HTMLButtonElement)

	chCmd = make(chan tagValue, 4)
	chStep = make(signalChannel, 16)
	chLog = make(signalChannel)

	go callServer()
	go doAnimation()

	// also probably reload (source and symbols) button, send 'L' tagvalue

	input_field := d.GetElementByID("indev").(*dom.HTMLInputElement)

	butt_step.AddEventListener("click", false, func(event dom.Event) {
		stepByStep = true
		traceEachLine = true
		chStep <- nil
		butt_trace.Style().SetProperty("color", "black", "")
		butt_run.Style().SetProperty("color", "black", "")
		butt_step.SetTextContent("Step")
		if askForMore {
			askForMore = false
			chCmd <- tagValue{C: 'C'}
		}
	})

	butt_trace.AddEventListener("click", false, func(event dom.Event) {
		stepByStep = false
		traceEachLine = true
		chStep <- nil
		butt_trace.Style().SetProperty("color", "red", "")
		butt_run.Style().SetProperty("color", "black", "")
		butt_step.SetTextContent("STOP")
		if askForMore {
			askForMore = false
			chCmd <- tagValue{C: 'C'}
		}
	})

	butt_run.AddEventListener("click", false, func(event dom.Event) {
		stepByStep = false
		traceEachLine = false
		chStep <- nil
		butt_run.Style().SetProperty("color", "red", "")
		butt_trace.Style().SetProperty("color", "black", "")
		butt_step.SetTextContent("STOP")
		if askForMore {
			askForMore = false
			chCmd <- tagValue{C: 'C'}
		}
	})

	butt_restart.AddEventListener("click", false, func(event dom.Event) {
		chCmd <- tagValue{C: 'R'}
	})

	butt_exit.AddEventListener("click", false, func(event dom.Event) {
		// runAnimation = false // never exit until closed by the browser
		chCmd <- tagValue{C: 'X'}
	})

	input_field.AddEventListener("keyup", false, func(event dom.Event) {
		key := event.(*dom.KeyboardEvent)
		inpval := input_field.Value
		if key.KeyCode == 13 && inpval != "" && askForInput {
			chCmd <- tagValue{C: 'I', V: inpval}
			input_field.Value = ""
			askForInput = false
		}
	})
}
