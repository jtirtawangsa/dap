package ui

import (
	//	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/cathalgarvey/fmtless/encoding/json"
)

func indexHandler(asset string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// log.Print("index requested")
		fhtml, err := os.Open(asset + "/assets/index.html")
		if err != nil {
			log.Fatal(err)
		}
		html, err := ioutil.ReadAll(fhtml)
		if err != nil {
			log.Fatal(err)
		}
		fhtml.Close()
		fmt.Fprintf(w, string(html))
	})
}

func traceHandler(chlog <-chan []byte, chcmd chan<- []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// log.Print("trace requested")
		if cmdJson, err := ioutil.ReadAll(r.Body); err != nil {
			log.Print(err)
		} else {
			// log.Print(string(cmdJson))
			chcmd <- cmdJson
		}
		traceJson := <-chlog
		w.Write(traceJson)
		// log.Print(string(traceJson))
	})
}

func ServeWeb(uri, asset string, chlog <-chan []byte, chcmd chan<- []byte) {
	log.Println("Listening http request at", uri)
	/*
		l, err := net.Listen("tcp", uri)
		if err != nil {
			panic(err)
		}
		server := &http.Server{
			ReadTimeout:    60 * time.Second,
			WriteTimeout:   60 * time.Second,
			MaxHeaderBytes: 1 << 16}
	*/
	http.Handle("/", indexHandler(asset))
	http.Handle("/daptrace", traceHandler(chlog, chcmd))
	http.Handle("/assets/", http.FileServer(http.Dir(asset)))
	// server.Serve(l)
	http.ListenAndServe(uri, nil)
}

type tagValue struct {
	C byte
	V interface{}
}

/* responds:
I value ; input
X       ; terminate
R       ; restart from the beginning
C       ; continue please
L       ; ask for source program and variable offsets (not used in console)
*/
func Serve(chlog <-chan []byte, chcmd chan<- []byte) {
	var (
		trace    []tagValue
		lastline int
		respond  tagValue
		done     = false
	)
	respond.C = 'R' // start to run
	if respJson, err := json.Marshal(respond); err != nil {
		log.Printf("DAP.w %v -- Fail to marshal load request", lastline)
	} else {
		// log.Print("UI service activating")
		chcmd <- respJson
		// log.Print("UI service started")
	}
	// log.Print("UI service ready")
	for !done {
		respond.C = 0 // non command
		// log.Print("UI service receiving")
		traceJson := <-chlog
		json.Unmarshal(traceJson, &trace)
		// log.Print("UI service received:", trace)
		for _, atr := range trace {
			switch atr.C {
			case 'L':
				// fmt.Printf("DAP.w %v -- executing line %v\n", atr.V, atr.V)
				lastline = int(atr.V.(float64))

			case 'O':
				fmt.Printf("DAP.w %v -- output: %v\n", lastline, atr.V)

			case 'V':
				// fmt.Printf("DAP.w %v -- storing %v\n", lastline, atr.V)

			case 'I':
				fmt.Printf("DAP.w %v -- input ", lastline)
				var val string
				fmt.Scanln(&val)
				respond.C = 'I'
				respond.V = val

			case 'C':
				fmt.Printf("DAP.w %v -- continue? (_C_,X,R) ", lastline)
				for {
					var val string
					fmt.Scanln(&val)
					if len(val) > 0 {
						respond.C = val[0]
					} else {
						respond.C = 'C'
					}
					// fmt.Scanf("%c", &respond.C)
					if respond.C > 31 {
						break
					}
				}
				respond.V = nil

			case 'X':
				fmt.Printf("DAP.w %v -- error %v (C,_X_,R) ", lastline, atr.V)
				var val string
				fmt.Scanln(&val)
				if len(val) > 0 {
					respond.C = val[0]
				} else {
					respond.C = 'X'
				}
				//fmt.Scanf("%c", &respond.C)
				respond.V = nil

			case 'E':
				fmt.Printf("DAP.w %v -- terminate (_X_,R) ", lastline)
				var val string
				fmt.Scanln(&val)
				if len(val) > 0 {
					respond.C = val[0]
				} else {
					respond.C = 'X'
				}
				//fmt.Scanf("%c", &respond.C)
				respond.V = nil

			case 'P':
				// log.Print("retrieve sources\n" + atr.V.(string))
				fmt.Printf("DAP.w %v -- sources (X,_R_) ", lastline)
				var val string
				fmt.Scanln(&val)
				if len(val) > 0 {
					respond.C = val[0]
				} else {
					respond.C = 'R'
				}
				// fmt.Scanf("%c", &respond.C)
				respond.V = nil

			case 'D':
				// log.Print("retrieve data offsets\n")
				// log.Print(atr.V)
				fmt.Printf("DAP.w %v -- offsets (X,_R_) ", lastline)
				var val string
				fmt.Scanln(&val)
				if len(val) > 0 {
					respond.C = val[0]
				} else {
					respond.C = 'R'
				}
				// fmt.Scanf("%c", &respond.C)
				respond.V = nil

			case 'A':
				// log.Print("retrieve (line,offset) pairs\n")
				// log.Print(atr.V)
				fmt.Printf("DAP.w %v -- pairs (X,_R_) ", lastline)
				var val string
				fmt.Scanln(&val)
				if len(val) > 0 {
					respond.C = val[0]
				} else {
					respond.C = 'R'
				}
				// fmt.Scanf("%c", &respond.C)
				respond.V = nil
			}
			if respond.C == 'X' {
				done = true
				//close(chlog)
			}
		}
		if respond.C == 0 { // no command
		} else if respJson, err := json.Marshal(respond); err != nil {
			log.Printf("DAP.w %v -- Fail to marshal respond", lastline)
		} else {
			// log.Printf("UI service sending %c", respond.C)
			chcmd <- respJson
			// log.Print("UI service sent")
		}
	}
	log.Print("DAP.w *** User interface done")
}
