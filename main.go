package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/go-delve/delve/service/api"
	"github.com/go-delve/delve/service/rpc2"
	"github.com/peterh/liner"
	"golang.org/x/term"
)

type DebugState struct {
	File        string
	Line        int
	GoroutineID int64
	VarsLocal   []api.Variable
	VarsFunc    []api.Variable
	Watch       []api.Variable
	Breakpoints []*api.Breakpoint
	Assembly    []api.AsmInstruction
}

type TermState struct {
	LastCommand     string
	PaneSource      bool
	PaneAssembly    bool
	PaneVars        bool
	PaneBreakpoints bool
	PaneWatch       bool
	HeightSource    int
	HeightAssembly  int
	Watch           []string
	Breakpoints     map[string][]int
}

var normalLoadConfig = api.LoadConfig{
	FollowPointers:     true,
	MaxVariableRecurse: 1,
	MaxStringLen:       64,
	MaxArrayValues:     64,
	MaxStructFields:    -1,
}

var userHomeDir string
var filecache = map[string][]string{}

func main() {
	userHomeDir, _ = os.UserHomeDir()
	width, _, _ := term.GetSize(1)
	term := LoadTermState()
	dlv := rpc2.NewClient("127.0.0.1:6060")

	for file, lines := range term.Breakpoints {
		for _, line := range lines {
			dlv.CreateBreakpoint(&api.Breakpoint{
				File: file,
				Line: line,
			})
		}
	}

	for {
		state, err := GetState(dlv, term.Watch)
		if err != nil {
			fmt.Println(err)
			return
		}

		w := &strings.Builder{}
		w.Write(ClearScreen)
		w.Write(ClearScreenFull)

		if term.PaneSource {
			printSource(w, state.File, state.Line, state.Breakpoints, width, term.HeightSource)
		}
		if term.PaneAssembly {
			printAssembly(w, state.Assembly, width, term.HeightAssembly)
		}
		if term.PaneVars {
			printVars(w, state.VarsLocal, state.VarsFunc, width)
		}
		if term.PaneWatch {
			printWatch(w, state.Watch, width)
		}
		if term.PaneBreakpoints {
			printBreakpoints(w, state.Breakpoints, width)
		}

		printLine(w, width)
		os.Stdout.WriteString(w.String())

		cmd, err := ReadLine()
		if err != nil {
			break
		}

		var ok bool
		term, ok = Update(term, state, dlv, cmd)
		if !ok {
			return
		}
		SaveTermState(term)
	}

}

func Update(term TermState, debug DebugState, dlv *rpc2.RPCClient, cmd string) (TermState, bool) {
	switch cmd {
	case "c", "continue":
		s := <-dlv.Continue()
		if s.Exited {
			return term, false
		}
	case "n", "next":
		dlv.Next()
	case "s", "step":
		dlv.Step()
	case "so", "stepout":
		dlv.StepOut()
	case "ni", "nexti":
		dlv.StepInstruction(true)
	case "si", "stepi":
		dlv.StepInstruction(false)
	case "q", "quit":
		return term, false
	case "pane src", "pane source":
		term.PaneSource = !term.PaneSource
	case "pane asm", "pane assembly":
		term.PaneAssembly = !term.PaneAssembly
	case "pane vars", "pane variables":
		term.PaneVars = !term.PaneVars
	case "pane bp", "pane breakpoints":
		term.PaneBreakpoints = !term.PaneBreakpoints
	case "pane watch":
		term.PaneWatch = !term.PaneWatch
	case "grow src", "grow source":
		term.HeightSource += 2
	case "grow asm", "grow assembly":
		term.HeightAssembly += 2
	case "shrink src", "shrink source":
		term.HeightSource = max(1, term.HeightSource-2)
	case "shrink asm", "shrink assembly":
		term.HeightAssembly = max(1, term.HeightAssembly-2)
	case "":
		return Update(term, debug, dlv, term.LastCommand)
	default:
		if strings.HasPrefix(cmd, "c ") {
			cmd = strings.Replace(cmd, "c ", "clear ", 1)
		} else if strings.HasPrefix(cmd, "b ") {
			cmd = strings.Replace(cmd, "b ", "break ", 1)
		} else if strings.HasPrefix(cmd, "w ") {
			cmd = strings.Replace(cmd, "w ", "watch ", 1)
		} else if strings.HasPrefix(cmd, "uw ") {
			cmd = strings.Replace(cmd, "uw ", "unwatch ", 1)
		}

		if after, ok := strings.CutPrefix(cmd, "break "); ok {
			if file, linestr, ok := strings.Cut(after, ":"); ok {
				line, _ := strconv.Atoi(linestr)
				if !strings.Contains(file, "/") {
					file = filepath.Join(filepath.Dir(debug.File), file)
				}
				bp, err := dlv.CreateBreakpoint(&api.Breakpoint{
					File: file,
					Line: line,
				})
				if err == nil {
					term.Breakpoints[bp.File] = append(term.Breakpoints[bp.File], line)
				}
			} else {
				line, err := strconv.Atoi(after)
				if err != nil {
					bp, err := dlv.CreateBreakpoint(&api.Breakpoint{FunctionName: after})
					if err == nil {
						term.Breakpoints[bp.File] = append(term.Breakpoints[bp.File], bp.Line)
					}
				} else {
					bp, err := dlv.CreateBreakpoint(&api.Breakpoint{
						File: debug.File,
						Line: line,
					})
					if err == nil {
						term.Breakpoints[bp.File] = append(term.Breakpoints[bp.File], line)
					}
				}
			}
		} else if after, ok := strings.CutPrefix(cmd, "clear "); ok {
			id, _ := strconv.Atoi(after)
			bp, err := dlv.ClearBreakpoint(id)
			if err == nil {
				term.Breakpoints[bp.File] = slices.DeleteFunc(term.Breakpoints[bp.File], func(line int) bool {
					return line == bp.Line
				})
				if len(term.Breakpoints[bp.File]) == 0 {
					delete(term.Breakpoints, bp.File)
				}
			}
		} else if after, ok := strings.CutPrefix(cmd, "watch "); ok {
			term.Watch = append(term.Watch, after)
		} else if after, ok := strings.CutPrefix(cmd, "unwatch "); ok {
			if id, err := strconv.Atoi(after); err == nil {
				idx := id - 1
				if idx >= 0 && idx < len(term.Watch) {
					term.Watch = slices.Delete(term.Watch, idx, idx+1)
				}
			} else {
				for i := range term.Watch {
					if term.Watch[i] == after {
						term.Watch = slices.Delete(term.Watch, i, i+1)
					}
				}
			}
		}

	}

	term.LastCommand = cmd
	return term, true
}

func ReadLine() (string, error) {
	os.Stdout.Write(ColorReset)

	histFile := filepath.Join(userHomeDir, ".godhistory")

	ln := liner.NewLiner()
	defer ln.Close()

	if f, err := os.Open(histFile); err == nil {
		ln.ReadHistory(f)
		f.Close()
	}

	cmd, err := ln.Prompt(">>> ")
	if err != nil {
		return "", err
	}
	cmd = strings.TrimSpace(cmd)

	if cmd != "" && cmd != "q" && cmd != "quit" {
		ln.AppendHistory(cmd)
		if f, err := os.Create(histFile); err == nil {
			ln.WriteHistory(f)
			f.Close()
		}
	}

	return cmd, err
}

func LoadTermState() TermState {
	defaultState := TermState{
		LastCommand:     "?",
		PaneSource:      true,
		PaneAssembly:    true,
		PaneVars:        true,
		PaneBreakpoints: true,
		PaneWatch:       true,
		HeightSource:    9,
		HeightAssembly:  9,
		Watch:           []string{},
		Breakpoints:     map[string][]int{},
	}

	file := filepath.Join(userHomeDir, ".godconfig")
	data, err := os.ReadFile(file)
	if err != nil {
		return defaultState
	}

	var state TermState
	if err := json.Unmarshal(data, &state); err != nil {
		return defaultState
	}

	return state
}

func SaveTermState(state TermState) {
	state.LastCommand = "?"

	for file := range state.Breakpoints {
		slices.Sort(state.Breakpoints[file])
	}

	file := filepath.Join(userHomeDir, ".godconfig")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(file, data, 0o644)
}

func GetState(dlv *rpc2.RPCClient, watchExpr []string) (DebugState, error) {
	ds, err := dlv.GetState()
	if err != nil {
		return DebugState{}, err
	}

	var state DebugState
	state.GoroutineID = ds.CurrentThread.GoroutineID
	state.File = ds.CurrentThread.File
	state.Line = ds.CurrentThread.Line
	state.Breakpoints, err = dlv.ListBreakpoints(true)
	if err != nil {
		return DebugState{}, err
	}
	sort.Slice(state.Breakpoints, func(i, j int) bool {
		return state.Breakpoints[i].ID < state.Breakpoints[j].ID
	})

	evalScope := api.EvalScope{GoroutineID: state.GoroutineID}

	if state.GoroutineID != 0 {
		state.VarsLocal, err = dlv.ListLocalVariables(evalScope, normalLoadConfig)
		if err != nil {
			return DebugState{}, err
		}
		state.VarsFunc, err = dlv.ListFunctionArgs(evalScope, normalLoadConfig)
		if err != nil {
			return DebugState{}, err
		}
		state.Assembly, err = dlv.DisassemblePC(evalScope, ds.CurrentThread.PC, api.GoFlavour)
		if err != nil {
			return DebugState{}, err
		}
		for _, expr := range watchExpr {
			v, err := dlv.EvalVariable(evalScope, expr, normalLoadConfig)
			if err != nil {
				state.Watch = append(state.Watch, api.Variable{Name: expr})
			} else {
				state.Watch = append(state.Watch, *v)
			}
		}
	}

	return state, nil
}

func printBreakpoints(w *strings.Builder, breakpoints []*api.Breakpoint, width int) {
	printHeader(w, "Breakpoints", width)
	tabw := tabwriter.NewWriter(w, 0, 1, 1, ' ', 0)

	for _, bp := range breakpoints {
		if bp.ID > 0 {
			fmt.Fprintf(tabw, "%s%d\t%s%s:%d (%s)\n", ColorFGGray, bp.ID, ColorFGWhite, filepath.Base(bp.File), bp.Line, bp.FunctionName)

		}
	}

	tabw.Flush()
}

func printWatch(w *strings.Builder, watch []api.Variable, width int) {
	printHeader(w, "Watch", width)

	for i, v := range watch {
		w.Write(ColorFGGray)
		w.WriteString(strconv.Itoa(i + 1))
		w.WriteString(" ")
		w.Write(ColorFGWhite)
		w.WriteString(v.Name)
		w.Write(ColorFGGray)
		w.WriteString(" = ")
		w.Write(ColorFGWhite)

		s := v.SinglelineStringWithShortTypes()
		if len(s) > width-len(v.Name)-7 {
			s = s[:width-len(v.Name)-7]
		}

		w.WriteString(s)
		w.WriteString("\n")
	}

}

func printAssembly(w *strings.Builder, instructions api.AsmInstructions, width, height int) {
	printHeader(w, "Assembly", width)
	w.Write(ColorFGGray)

	pc := 0
	for i, inst := range instructions {
		if inst.AtPC {
			pc = i
			break
		}
	}

	start := max(0, pc-height/2)
	end := min(start+height, len(instructions))

	tabw := tabwriter.NewWriter(w, 24, 1, 4, ' ', 0)

	for i := start; i < end; i++ {
		inst := instructions[i]

		// PC
		if inst.AtPC {
			tabw.Write(ColorFGCyan)
		} else if inst.Breakpoint {
			tabw.Write(ColorFGMagenta)
		} else {
			tabw.Write(ColorFGGray)
		}
		fmt.Fprintf(tabw, "0x%x    ", inst.Loc.PC)

		// Bytes
		tabw.Write(ColorFGGray)
		for i, b := range inst.Bytes {
			if i == len(inst.Bytes)-1 {
				fmt.Fprintf(tabw, "%02x", b)
			} else {
				fmt.Fprintf(tabw, "%02x ", b)
			}
		}
		tabw.Write([]byte("\t"))

		// Text
		if inst.AtPC {
			tabw.Write(ColorFGCyan)
		} else {
			tabw.Write(ColorFGWhite)
		}
		segs := strings.SplitN(inst.Text, " ", 2)
		tabw.Write([]byte(segs[0]))
		tabw.Write([]byte("\t"))

		if len(segs) == 2 {
			tabw.Write([]byte(segs[1]))
			tabw.Write([]byte("\t"))
		}

		// Source
		fmt.Fprintf(tabw, "%s:%d\n", filepath.Base(inst.Loc.File), inst.Loc.Line)
	}
	tabw.Flush()
}

func printVars(w *strings.Builder, locals, funcs []api.Variable, width int) {
	printHeader(w, "Variables", width)

	vars := [][]api.Variable{funcs, locals}
	name := []string{"fun ", "loc "}

	for i := range vars {
		for _, v := range vars[i] {
			w.Write(ColorFGGray)
			w.WriteString(name[i])
			w.Write(ColorFGWhite)
			w.WriteString(v.Name)
			w.Write(ColorFGGray)
			w.WriteString(" = ")
			w.Write(ColorFGWhite)

			s := v.SinglelineStringWithShortTypes()
			if len(s) > width-len(v.Name)-7 {
				s = s[:width-len(v.Name)-7]
			}

			w.WriteString(s)
			w.WriteString("\n")
		}
	}
}

func printSource(w *strings.Builder, file string, pc int, breakpoints []*api.Breakpoint, width, height int) {
	printHeader(w, "Source", width)

	bpmap := make(map[int]*api.Breakpoint, len(breakpoints))
	for _, bp := range breakpoints {
		if bp.File == file {
			bpmap[bp.Line-1] = bp
		}
	}

	lines := readFileLines(file)
	currLine := pc - 1

	start := max(0, currLine-height/2)
	end := min(start+height, len(lines))
	digits := max(3, numDigits(end))
	iotaBuf := make([]byte, digits)
	for i := range digits {
		iotaBuf[i] = ' '
	}

	for i := start; i < end; i++ {
		iota(iotaBuf, i+1)
		if i == currLine {
			w.Write(ColorFGCyan)
		} else if _, ok := bpmap[i]; ok {
			w.Write(ColorFGMagenta)
		} else {
			w.Write(ColorFGGray)
		}

		w.Write(iotaBuf)
		w.WriteString(" ")
		if i == currLine {
			w.Write(ColorFGCyan)
		} else {
			w.Write(ColorFGWhite)
		}

		if len(lines[i]) > width-3 {
			w.WriteString(lines[i][:width-3-digits])
		} else {
			w.WriteString(lines[i])
		}
		w.WriteString("\n")
	}
}

func printHeader(w *strings.Builder, title string, width int) {
	prefix := 3

	w.Write(ColorFGCyan)
	w.WriteString(strings.Repeat("━", prefix))
	w.Write(ColorFGMagenta)
	w.WriteString(" ")
	w.WriteString(title)
	w.WriteString(" ")
	w.Write(ColorFGCyan)
	w.WriteString(strings.Repeat("━", width-len(title)-2-prefix))
}

func printLine(w *strings.Builder, width int) {
	w.Write(ColorFGCyan)
	w.WriteString(strings.Repeat("━", width))
	w.WriteString("\n")
}

func numDigits(i int) int {
	if i == 0 {
		return 1
	}
	count := 0
	for i != 0 {
		i /= 10
		count++
	}
	return count
}

func iota(buf []byte, n int) {
	for i := len(buf) - 1; i >= 0; i-- {
		if n > 0 {
			buf[i] = byte(n%10) + '0'
			n = n / 10
		} else {
			buf[i] = ' '
		}
	}

}

func readFileLines(file string) []string {
	if lines, ok := filecache[file]; ok {
		return lines
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return []string{}
	}
	src := string(data)
	src = strings.ReplaceAll(src, "\t", "    ")
	lines := strings.Split(src, "\n")
	filecache[file] = lines
	return lines
}

var (
	AltScreen       = []byte("\033[?1049h")
	ExitAltScreen   = []byte("\033[?1049l")
	ClearScreen     = []byte("\033[2J")
	ClearScreenFull = []byte("\033[H")
	ClearLine       = []byte("\033[2K")
	ResetCursor     = []byte("\033[1;1H")
	ShowCursor      = []byte("\033[?25h")
	HideCursor      = []byte("\033[?25l")

	ColorReset     = []byte("\033[m")
	ColorFGGray    = []byte("\033[38;37m")
	ColorFGBlack   = []byte("\033[38;90m")
	ColorFGRed     = []byte("\033[38;91m")
	ColorFGGreen   = []byte("\033[38;92m")
	ColorFGYellow  = []byte("\033[38;93m")
	ColorFGBlue    = []byte("\033[38;94m")
	ColorFGMagenta = []byte("\033[38;95m")
	ColorFGCyan    = []byte("\033[38;96m")
	ColorFGWhite   = []byte("\033[38;97m")
)
