package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/pronskiy/php-debugger/cli/dbgp"
)

// breakpointFlag implements flag.Value for multiple -breakpoint flags
type breakpointFlag struct {
	file  string
	lines []int
}

type breakpointFlags []breakpointFlag

func (b *breakpointFlags) String() string {
	return fmt.Sprintf("%v", *b)
}

func (b *breakpointFlags) Set(value string) error {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid breakpoint format: %s (expected file:line[,line,...])", value)
	}
	file := parts[0]
	var lines []int
	for _, ls := range strings.Split(parts[1], ",") {
		l, err := strconv.Atoi(strings.TrimSpace(ls))
		if err != nil {
			return fmt.Errorf("invalid line number: %s", ls)
		}
		lines = append(lines, l)
	}
	*b = append(*b, breakpointFlag{file: file, lines: lines})
	return nil
}

var (
	flagPort       int
	flagTimeout    time.Duration
	flagBreakpoint breakpointFlags
	flagRaw        bool
)

func init() {
	flag.IntVar(&flagPort, "port", 9003, "TCP port for DBGp protocol connection")
	flag.DurationVar(&flagTimeout, "timeout", 30*time.Second, "Maximum time to wait for PHP connection")
	flag.Var(&flagBreakpoint, "breakpoint", "Set breakpoint (can be repeated). Format: file.php:line or file.php:line1,line2,line3")
	flag.BoolVar(&flagRaw, "raw", false, "Output raw XML responses below variables")
}

type Debugger struct {
	listener net.Listener
	conn     net.Conn
	reader   *bufio.Reader
	writer   io.Writer

	transID    int
	initPacket string
	fileURI    string
}

func NewDebugger(port int) (*Debugger, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return nil, err
	}
	return &Debugger{listener: listener}, nil
}

func (d *Debugger) Addr() string {
	return d.listener.Addr().String()
}

func (d *Debugger) WaitForConnection(timeout time.Duration) error {
	tcpListener := d.listener.(*net.TCPListener)
	tcpListener.SetDeadline(time.Now().Add(timeout))

	conn, err := d.listener.Accept()
	if err != nil {
		return err
	}

	d.conn = conn
	d.reader = bufio.NewReader(conn)
	d.writer = conn

	// Read init packet
	initData, err := d.readPacket()
	if err != nil {
		return fmt.Errorf("read init: %w", err)
	}

	d.initPacket = string(initData)

	// Parse fileuri from init
	if strings.Contains(d.initPacket, "fileuri=") {
		start := strings.Index(d.initPacket, `fileuri="`)
		if start != -1 {
			start += 9
			end := strings.Index(d.initPacket[start:], `"`)
			if end != -1 {
				d.fileURI = d.initPacket[start : start+end]
			}
		}
	}

	return nil
}

func (d *Debugger) readPacket() ([]byte, error) {
	line, err := d.reader.ReadString('\x00')
	if err != nil {
		return nil, err
	}

	line = strings.TrimSuffix(line, "\x00")
	length, err := strconv.Atoi(line)
	if err != nil {
		return nil, fmt.Errorf("parse length: %w", err)
	}

	data := make([]byte, length)
	_, err = io.ReadFull(d.reader, data)
	if err != nil {
		return nil, err
	}

	b, err := d.reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if b != '\x00' {
		return nil, fmt.Errorf("expected NULL, got %x", b)
	}

	return data, nil
}

func (d *Debugger) sendCommand(cmd string) (string, error) {
	// Check if command already has -i
	if !strings.Contains(cmd, "-i ") {
		d.transID++
		cmd = fmt.Sprintf("%s -i %d", cmd, d.transID)
	} else {
		parts := strings.Split(cmd, "-i ")
		if len(parts) > 1 {
			idStr := strings.Fields(parts[1])[0]
			id, _ := strconv.Atoi(idStr)
			d.transID = id
		}
	}

	packet := fmt.Sprintf("%d\x00%s\x00", len(cmd), cmd)
	_, err := d.writer.Write([]byte(packet))
	if err != nil {
		return "", err
	}

	// Read response (skip async responses without transaction_id)
	for {
		respData, err := d.readPacket()
		if err != nil {
			return "", err
		}

		resp := string(respData)
		expectedID := fmt.Sprintf(`transaction_id="%d"`, d.transID)
		if strings.Contains(resp, expectedID) {
			return resp, nil
		}
	}
}

func (d *Debugger) Close() {
	if d.conn != nil {
		d.conn.Close()
	}
	if d.listener != nil {
		d.listener.Close()
	}
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `PHP Debugger CLI - DBGp Protocol Debug Tool

DESCRIPTION
  A command-line debugger for PHP applications using the DBGp protocol.
  Listens for incoming connections from PHP's Xdebug or php-debugger extension
  to set breakpoints, inspect variables, and step through code execution.

USAGE
  dbg [options]

OPTIONS
  -port <int>           TCP port for DBGp connection (default: 9003)
  -timeout <duration>   Time to wait for PHP connection before exiting (default: 30s)
  -breakpoint <spec>    Set breakpoint, can be repeated (see format below)
  -raw                  Output raw XML responses below parsed variables

BREAKPOINT FORMAT
  file.php:42              Single breakpoint at line 42
  file.php:10,20,30        Multiple lines in same file
  path/to/file.php:100     With relative or absolute path

  The -breakpoint flag can be specified multiple times:
    dbg -breakpoint app.php:10 -breakpoint lib.php:20

EXAMPLES
  # Listen for connections with breakpoints set
  dbg -breakpoint app.php:42

  # Multiple breakpoints in different files
  dbg -breakpoint Controller.php:10,20,30 -breakpoint Model.php:50

  # Custom port
  dbg -port 9003

  # With raw XML output for debugging
  dbg -raw -breakpoint app.php:42

RUNNING PHP SCRIPTS
  Optionally, you can specify a PHP script to run automatically:

  dbg -breakpoint app.php:42 app.php
  dbg -breakpoint Controller.php:10 app.php [script args...]

  This sets XDEBUG_SESSION=1 and XDEBUG_CONFIG=client_port=<port> before
  running the script. For web requests or external processes, omit the script.

PROTOCOL
  Uses DBGp (Debug Generic Protocol) - the same protocol used by Xdebug.
  PHP connects as a client to this debugger which acts as a server.

`)
	}
	flag.Parse()

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nInterrupted")
		cancel()
	}()

	dbg, err := NewDebugger(flagPort)
	if err != nil {
		return fmt.Errorf("create listener: %w", err)
	}
	defer dbg.Close()

	fmt.Printf("Listening on %s\n", dbg.Addr())

	// Parse breakpoints from flags
	var breakpoints []breakpoint
	for _, bp := range flagBreakpoint {
		for _, line := range bp.lines {
			breakpoints = append(breakpoints, breakpoint{file: bp.file, line: line})
		}
	}

	// Get PHP script from args
	args := flag.Args()
	var phpScript string
	var phpArgs []string
	for i, arg := range args {
		if strings.HasSuffix(arg, ".php") {
			phpScript = arg
			phpArgs = args[i+1:]
			break
		}
	}

	// Start PHP if script provided
	var phpCmd *exec.Cmd
	if phpScript != "" {
		if !filepath.IsAbs(phpScript) {
			phpScript, _ = filepath.Abs(phpScript)
		}
		phpCmd = exec.CommandContext(ctx, "php", append([]string{phpScript}, phpArgs...)...)
		phpCmd.Env = append(os.Environ(),
			"XDEBUG_SESSION=1",
			fmt.Sprintf("XDEBUG_CONFIG=client_port=%d", flagPort),
		)
		phpCmd.Stdout = os.Stdout
		phpCmd.Stderr = os.Stderr
	}

	if phpScript != "" {
		fmt.Printf("Starting: php %s\n", phpScript)
	} else {
		fmt.Println("Waiting for PHP to connect...")
	}

	// Wait for connection
	connChan := make(chan error, 1)
	go func() {
		connChan <- dbg.WaitForConnection(flagTimeout)
	}()

	if phpCmd != nil {
		time.Sleep(100 * time.Millisecond)
		if err := phpCmd.Start(); err != nil {
			return fmt.Errorf("start PHP: %w", err)
		}
	}

	select {
	case err := <-connChan:
		if err != nil {
			return fmt.Errorf("connection: %w", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	fmt.Printf("\n✓ Connected: %s\n", strings.TrimPrefix(dbg.fileURI, "file://"))

	// Enable features
	dbg.sendCommand("feature_set -n resolved_breakpoints -v 1")

	// Set breakpoints
	for _, bp := range breakpoints {
		bpFile := bp.file
		if !filepath.IsAbs(bpFile) {
			bpFile, _ = filepath.Abs(bpFile)
		}
		cmd := fmt.Sprintf("breakpoint_set -t line -f file://%s -n %d", bpFile, bp.line)
		resp, err := dbg.sendCommand(cmd)
		if err != nil {
			fmt.Printf("✗ %s:%d: %v\n", bpFile, bp.line, err)
			continue
		}
		if strings.Contains(resp, "error") {
			fmt.Printf("✗ %s:%d: error\n", bpFile, bp.line)
		} else {
			fmt.Printf("✓ Breakpoint: %s:%d\n", filepath.Base(bpFile), bp.line)
		}
	}

	// Run and wait for breakpoint
	fmt.Println("\n→ Running...")

	resp, err := dbg.sendCommand("run")
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	// Process breakpoints
	for strings.Contains(resp, `status="break"`) {
		file, line := parseMessage(resp)
		fmt.Printf("\n━━━ %s:%d ━━━\n", filepath.Base(file), line)

		// Get stack
		stackResp, _ := dbg.sendCommand("stack_get")
		fmt.Println("\nStack:")
		printStack(stackResp)

		// Get variables
		varsResp, _ := dbg.sendCommand("context_get -d 0 -c 0")
		vars := dbgp.ParseVariables(varsResp)
		fmt.Println("\nVariables:")
		for _, v := range vars {
			fmt.Println(dbgp.FormatVariable(v))
		}

		// Show raw XML if requested
		if flagRaw {
			fmt.Println("\nRaw XML:")
			fmt.Println(varsResp)
		}

		// Continue
		resp, err = dbg.sendCommand("run")
		if err != nil {
			return err
		}
	}

	if strings.Contains(resp, `status="stopping"`) || strings.Contains(resp, `status="stopped"`) {
		fmt.Println("\n✓ Done")
	}

	if phpCmd != nil {
		phpCmd.Wait()
	}

	return nil
}

type breakpoint struct {
	file string
	line int
}

func parseMessage(resp string) (string, int) {
	fileIdx := strings.Index(resp, `filename="`)
	if fileIdx == -1 {
		return "", 0
	}
	fileIdx += 10
	fileEnd := strings.Index(resp[fileIdx:], `"`)
	file := resp[fileIdx : fileIdx+fileEnd]
	file = strings.TrimPrefix(file, "file://")

	lineIdx := strings.Index(resp, `lineno="`)
	if lineIdx == -1 {
		return file, 0
	}
	lineIdx += 8
	lineEnd := strings.Index(resp[lineIdx:], `"`)
	line, _ := strconv.Atoi(resp[lineIdx : lineIdx+lineEnd])

	return file, line
}

func printStack(resp string) {
	for {
		stackIdx := strings.Index(resp, `<stack `)
		if stackIdx == -1 {
			break
		}

		endIdx := strings.Index(resp[stackIdx:], `></stack>`)
		if endIdx == -1 {
			endIdx = strings.Index(resp[stackIdx:], `/>`)
			if endIdx == -1 {
				break
			}
			endIdx += 2
		} else {
			endIdx += 9
		}

		stack := resp[stackIdx : stackIdx+endIdx]
		resp = resp[stackIdx+endIdx:]

		level := extractAttr(stack, "level")
		where := extractAttr(stack, "where")
		filename := extractAttr(stack, "filename")
		lineno := extractAttr(stack, "lineno")

		filename = strings.TrimPrefix(filename, "file://")

		if level != "" {
			fmt.Printf("#%s %s() at %s:%s\n", level, unescapeXML(where), filepath.Base(filename), lineno)
		}
	}
}

func extractAttr(s, name string) string {
	pattern := fmt.Sprintf(`%s="`, name)
	idx := strings.Index(s, pattern)
	if idx == -1 {
		return ""
	}
	idx += len(pattern)
	end := strings.Index(s[idx:], `"`)
	if end == -1 {
		return ""
	}
	return s[idx : idx+end]
}

func unescapeXML(s string) string {
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}
