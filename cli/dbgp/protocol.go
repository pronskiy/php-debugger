// Package dbgp implements the DBGp protocol for PHP debugging
package dbgp

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// charsetReader handles non-UTF-8 XML encodings by treating them as UTF-8
func charsetReader(charset string, input io.Reader) (io.Reader, error) {
	// Treat all encodings as UTF-8 (DBGp typically uses ASCII-safe content anyway)
	if strings.EqualFold(charset, "iso-8859-1") ||
		strings.EqualFold(charset, "windows-1252") ||
		strings.EqualFold(charset, "utf-8") ||
		strings.EqualFold(charset, "us-ascii") {
		return input, nil
	}
	return nil, fmt.Errorf("unsupported charset: %s", charset)
}

// Status values
const (
	StatusStarting  = "starting"
	StatusStopping  = "stopping"
	StatusStopped   = "stopped"
	StatusRunning   = "running"
	StatusBreak     = "break"
	StatusDetached  = "detached"
)

// Breakpoint types
const (
	BreakpointLine        = "line"
	BreakpointConditional = "conditional"
	BreakpointCall        = "call"
	BreakpointReturn      = "return"
	BreakpointException   = "exception"
	BreakpointWatch       = "watch"
)

// InitPacket is sent by PHP when connection is established
type InitPacket struct {
	XMLName       xml.Name `xml:"init"`
	AppID         string   `xml:"appid,attr"`
	IDEKey        string   `xml:"idekey,attr"`
	Session       string   `xml:"session,attr"`
	Thread        string   `xml:"thread,attr"`
	Parent        string   `xml:"parent,attr"`
	Language      string   `xml:"language,attr"`
	Protocol      string   `xml:"protocol_version,attr"`
	FileURI       string   `xml:"fileuri"`
	EngineVersion string   `xml:"engine>version"`
}

// Response is the generic DBGp response
type Response struct {
	XMLName      xml.Name `xml:"response"`
	Command      string   `xml:"command,attr"`
	Transaction  int      `xml:"transaction_id,attr"`
	Status       string   `xml:"status,attr,omitempty"`
	Reason       string   `xml:"reason,attr,omitempty"`
	Success      string   `xml:"success,attr,omitempty"`
	BreakpointID int      `xml:"id,attr,omitempty"`

	// For breakpoint_set
	Breakpoint *BreakpointInfo `xml:"breakpoint,omitempty"`

	// For context_get
	Context    int        `xml:"context,attr,omitempty"`
	Properties []Property `xml:"property,omitempty"`

	// For stack_get
	Stack []StackFrame `xml:"stack,omitempty"`

	// For errors
	Error *Error `xml:"error,omitempty"`

	// Message for breakpoint hit
	Message *Message `xml:"xdebug\\:message,omitempty"`

	// Raw for debugging
	Raw string `xml:",innerxml"`
}

// Error represents a DBGp error
type Error struct {
	Code    int    `xml:"code,attr"`
	Message string `xml:"message"`
}

// Message contains breakpoint hit information
type Message struct {
	Filename string `xml:"filename,attr"`
	Lineno   int    `xml:"lineno,attr"`
}

// BreakpointInfo contains breakpoint details
type BreakpointInfo struct {
	ID          int    `xml:"id,attr"`
	Type        string `xml:"type,attr"`
	Filename    string `xml:"filename,attr"`
	Lineno      int    `xml:"lineno,attr"`
	State       string `xml:"state,attr"`
	Exception   string `xml:"exception,attr,omitempty"`
	Expression  string `xml:"expression,attr,omitempty"`
	HitCount    int    `xml:"hit_count,attr,omitempty"`
	HitValue    int    `xml:"hit_value,attr,omitempty"`
	Temporary   int    `xml:"temporary,attr,omitempty"`
}

// Property represents a variable
type Property struct {
	Name            string      `xml:"name,attr"`
	FullName        string      `xml:"fullname,attr"`
	Type            string      `xml:"type,attr"`
	ClassName       string      `xml:"classname,attr,omitempty"`
	Facet           string      `xml:"facet,attr,omitempty"`
	Size            int         `xml:"size,attr,omitempty"`
	Children        int         `xml:"children,attr,omitempty"`
	NumChildren     int         `xml:"numchildren,attr,omitempty"`
	Encoding        string      `xml:"encoding,attr,omitempty"`
	Value           string      `xml:",chardata"`
	ChildProperties []Property  `xml:"property,omitempty"`
}

// StackFrame represents a call stack entry
type StackFrame struct {
	Level     int    `xml:"level,attr"`
	Type      string `xml:"type,attr"`
	Filename  string `xml:"filename,attr"`
	Lineno    int    `xml:"lineno,attr"`
	Where     string `xml:"where,attr"`
	Cmmd      string `xml:"cmmd,attr,omitempty"`
}

// ParseInit parses the init packet from PHP
func ParseInit(data []byte) (*InitPacket, error) {
	var init InitPacket
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charsetReader
	if err := decoder.Decode(&init); err != nil {
		return nil, fmt.Errorf("parse init: %w", err)
	}
	return &init, nil
}

// ParseResponse parses a DBGp response
func ParseResponse(data []byte) (*Response, error) {
	var resp Response
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.CharsetReader = charsetReader
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &resp, nil
}

// ParseMessage extracts breakpoint hit info from response
func (r *Response) ParseMessage() (file string, line int) {
	if r.Message != nil {
		file = strings.TrimPrefix(r.Message.Filename, "file://")
		line = r.Message.Lineno
	}
	return
}

func formatValue(p Property) string {
	if p.Encoding == "base64" {
		return "<base64>"
	}

	if p.Type == "array" || p.Type == "object" {
		if p.Children > 0 {
			return fmt.Sprintf("%s(%d)", p.Type, p.Children)
		}
		return p.Type + "(0)"
	}

	if p.Value == "" {
		switch p.Type {
		case "null":
			return "null"
		case "bool":
			return "false"
		case "string":
			return `""`
		default:
			return p.Type
		}
	}

	// Truncate long values
	if len(p.Value) > 100 {
		return p.Value[:97] + "..."
	}

	return p.Value
}

// FormatStack formats the call stack for display
func FormatStack(frames []StackFrame) []string {
	lines := make([]string, len(frames))
	for i, f := range frames {
		file := strings.TrimPrefix(f.Filename, "file://")
		lines[i] = fmt.Sprintf("#%d %s() at %s:%d", f.Level, f.Where, file, f.Lineno)
	}
	return lines
}

// FormatFileURI converts file:// URI to path
func FormatFileURI(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}

// MakeFileURI converts path to file:// URI
func MakeFileURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	return "file://" + path
}

// ParseBreakpointSpec parses "file.php:42" or "file.php:42,55,60"
func ParseBreakpointSpec(spec string) (file string, lines []int, err error) {
	parts := strings.Split(spec, ":")
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid breakpoint spec: %s (expected file:line)", spec)
	}

	file = parts[0]
	lineStrs := strings.Split(parts[1], ",")
	lines = make([]int, 0, len(lineStrs))

	for _, ls := range lineStrs {
		l, err := strconv.Atoi(strings.TrimSpace(ls))
		if err != nil {
			return "", nil, fmt.Errorf("invalid line number: %s", ls)
		}
		lines = append(lines, l)
	}

	return file, lines, nil
}
