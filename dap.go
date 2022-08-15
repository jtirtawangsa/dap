package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"dap/emulator"
	"dap/parser"
	"dap/scanner"
	"dap/ui"
)

var (
	dapAnimate  bool
	dapConsole  bool
	dapRun      bool
	dapCompile  bool
	dapAssembly bool
	listenPort  string
	dapSrcFile  string
	dapDestFile string
	dapAssets   string
	dapSource   bool = false
	dapSymbolic bool = false
	dapInternal bool = false
	dapSteps    int
)

func dapHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), `
This is a compiler, assembler, and runtime for a DAP-based language
It is used to study the trace and the effect of a program execution
Use the browser to invoke the user interface.
To store compiled codes, use file with extension .s4041
To store assembled codes, use file with extension .i4041
%s [-animate [-l <:port>]|-console|-run] [[-compile|-assembly] -o <destfile.ext]] <source program.dap>
`, os.Args[0])
	flag.PrintDefaults()
}

func validArgs() (ok bool) {
	ok = true
	flag.Usage = func() {
		dapHelp()
	}
	flag.BoolVar(&dapAnimate, "animate", false, "Animate (instead of run) the codes")
	flag.BoolVar(&dapAnimate, "emulate", false, "Animate (instead of run) the codes")
	flag.BoolVar(&dapConsole, "console", false, "Run the codes using animate protocol")
	flag.StringVar(&listenPort, "l", ":2345", "Animate at this HTTP port")
	flag.StringVar(&dapDestFile, "o", "", "Output of compiled (.s4041)/assembled (.i4041) codes")
	flag.BoolVar(&dapRun, "run", false, "Run (instead of animate) the codes")
	flag.IntVar(&dapSteps, "steps", 1000000, "Number of internal code execution")
	flag.StringVar(&dapAssets, "asset", "ui", "Folder where asset folder is located")
	flag.Parse()

	dapSrcFile = flag.Arg(0)
	lSrc := len(dapSrcFile)
	if lSrc >= 5 && dapSrcFile[lSrc-4:] == ".dap" {
		dapSource = true
	} else if lSrc < 7 {
		dapHelp()
		fmt.Fprintln(flag.CommandLine.Output(), "Non .dap source file mast have either .s4041 or .i4041 extension")
		ok = false
	} else if dapSrcFile[lSrc-6:] == ".s4041" {
		dapSymbolic = true
	} else if dapSrcFile[lSrc-6:] == ".i4041" {
		dapInternal = true
	} else {
		dapHelp()
		fmt.Fprintln(flag.CommandLine.Output(), "Source file must have either .dap, .s4041, or .i4041 extension")
		ok = false
	}
	if !(dapSource || dapSymbolic || dapInternal) {
		dapHelp()
		fmt.Fprintln(flag.CommandLine.Output(), "Source file must be given")
		ok = false
	}

	lDest := len(dapDestFile)
	if lDest >= 7 {
		dapCompile = dapDestFile[lDest-6:] == ".s4041"
		dapAssembly = dapDestFile[lDest-6:] == ".i4041"
	}
	if lDest > 0 && !dapCompile && !dapAssembly {
		fmt.Fprintln(flag.CommandLine.Output(), "File name extension for symbolic codes is '.s4041 and for internal code is .i4041")
		ok = false
	}

	if dapAnimate && dapRun {
		fmt.Fprintf(flag.CommandLine.Output(), "Use either -animate or -run to execute the compiled codes\n")
		ok = false
	}
	return
}

func performAnimation() {
	chlog := make(chan []byte, 8)
	chcmd := make(chan []byte, 8)
	chint := make(chan os.Signal)
	go ui.ServeWeb(listenPort, dapAssets, chlog, chcmd)
	emulator.Wemulate(dapSrcFile, dapSteps, chint, chlog, chcmd)
	signal.Notify(chint, syscall.SIGINT, syscall.SIGTERM)
	<-chint
	log.Print("DAP.m * Animation ends")
}

func openConsole() {
	chlog := make(chan []byte, 8)
	chcmd := make(chan []byte, 8)
	chint := make(chan os.Signal)
	go ui.Serve(chlog, chcmd)
	emulator.Wemulate(dapSrcFile, dapSteps, chint, chlog, chcmd)
	signal.Notify(chint, syscall.SIGINT, syscall.SIGTERM)
	<-chint
	log.Print("DAP.m * Console ends")
}

func main() {
	if !validArgs() {
		log.Fatal("Check command line")
	}
	if dapSource {
		token := scanner.NewToken(dapSrcFile)
		parser.Compile(token)
		parser.ProcessSymbols()
		emulator.GenCodes()
	} else if dapSymbolic {
		emulator.LoadSymbols(dapSrcFile)
		emulator.GenCodes()
	} else if dapInternal {
		emulator.LoadCodes(dapSrcFile)
	}

	if dapCompile {
		emulator.SaveSymbols(dapDestFile)
	} else if dapAssembly {
		emulator.SaveCodes(dapDestFile)
	}
	if dapAnimate {
		if dapSource {
			performAnimation()
		} else {
			log.Print("At this time, only source can be emulated")
		}
	} else if dapConsole {
		openConsole()
	} else if dapRun {
		emulator.Emulate(dapSteps)
	} else if dapSource {
		emulator.SaveVariables(dapSrcFile + "sym")
	}
}
