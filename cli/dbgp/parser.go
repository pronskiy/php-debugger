package dbgp

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"strings"
)

// Variable represents a parsed variable
type Variable struct {
	Name  string
	Type  string
	Value string
	Level int // nesting level for indentation
}

// ContextResponse wraps the XML response for context_get
type ContextResponse struct {
	XMLName    xml.Name   `xml:"response"`
	Properties []Property `xml:"property"`
}

// ParseVariables extracts top-level variables and their immediate children from a DBGp context_get response
func ParseVariables(xmlData string) []Variable {
	var resp ContextResponse
	decoder := xml.NewDecoder(bytes.NewReader([]byte(xmlData)))
	decoder.CharsetReader = charsetReader

	if err := decoder.Decode(&resp); err != nil {
		return nil
	}

	var vars []Variable
	for _, p := range resp.Properties {
		// Only include top-level variables (no brackets or arrows in name)
		if containsAny(p.FullName, "[", "->") {
			continue
		}
		vars = append(vars, Variable{
			Name:  p.FullName,
			Type:  p.Type,
			Value: formatPropertyValue(p),
			Level: 0,
		})
		// Add immediate child properties for objects/arrays
		for _, child := range p.ChildProperties {
			vars = append(vars, Variable{
				Name:  child.FullName,
				Type:  child.Type,
				Value: formatPropertyValue(child),
				Level: 1,
			})
		}
	}
	return vars
}

// ParseAllVariables extracts all variables including nested ones
func ParseAllVariables(xmlData string) []Variable {
	var resp ContextResponse
	decoder := xml.NewDecoder(bytes.NewReader([]byte(xmlData)))
	decoder.CharsetReader = charsetReader

	if err := decoder.Decode(&resp); err != nil {
		return nil
	}

	return flattenProperties(resp.Properties)
}

// flattenProperties recursively flattens nested properties
func flattenProperties(props []Property) []Variable {
	var vars []Variable
	for _, p := range props {
		vars = append(vars, Variable{
			Name:  p.FullName,
			Type:  p.Type,
			Value: formatPropertyValue(p),
		})
		if len(p.ChildProperties) > 0 {
			vars = append(vars, flattenProperties(p.ChildProperties)...)
		}
	}
	return vars
}

// formatPropertyValue formats a property value for display
func formatPropertyValue(p Property) string {
	// Handle types with children (check Children attr, ChildProperties, or raw XML in Value)
	hasChildren := p.Children > 0 || len(p.ChildProperties) > 0 ||
		(len(p.Value) > 0 && p.Value[0] == '<')

	if hasChildren {
		count := p.NumChildren
		if count == 0 {
			count = len(p.ChildProperties)
		}
		if count == 0 {
			count = p.Children // fallback
		}
		if count == 0 {
			count = 1 // at least one if we detected children
		}
		if p.Type == "object" && p.ClassName != "" {
			return p.ClassName
		}
		return fmt.Sprintf("%s[%d]", p.Type, count)
	}

	// Decode base64 if needed
	content := p.Value
	if p.Encoding == "base64" && content != "" {
		if decoded, err := base64.StdEncoding.DecodeString(content); err == nil {
			content = string(decoded)
		}
	}

	return formatSimpleValue(p.Type, content, p.ClassName)
}

// formatSimpleValue formats a value for display
func formatSimpleValue(typ, content, classname string) string {
	switch typ {
	case "null", "uninitialized":
		return "null"
	case "bool":
		if content == "1" || content == "true" {
			return "true"
		}
		return "false"
	case "string":
		if content == "" {
			return `""`
		}
		if len(content) > 60 {
			return `"` + content[:55] + `..."`
		}
		return `"` + content + `"`
	case "int", "float":
		return content
	case "array":
		if classname != "" {
			return classname + "[]"
		}
		return "array[]"
	case "object":
		if classname != "" {
			return classname + "{}"
		}
		return "object{}"
	default:
		if content == "" {
			return "<" + typ + ">"
		}
		if len(content) > 60 {
			return content[:57] + "..."
		}
		return content
	}
}

// FormatVariable formats a variable for display
func FormatVariable(v Variable) string {
	indent := strings.Repeat("  ", v.Level)
	width := 30 - (v.Level * 2)
	return fmt.Sprintf("%s%-*s %-8s = %s", indent, width, v.Name, v.Type, v.Value)
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if bytes.Contains([]byte(s), []byte(sub)) {
			return true
		}
	}
	return false
}
