package dbgp

import (
	"os"
	"testing"
)

func TestFormatSimpleValue(t *testing.T) {
	tests := []struct {
		typ       string
		content   string
		classname string
		want      string
	}{
		{"string", "hello", "", `"hello"`},
		{"string", "", "", `""`},
		{"int", "42", "", "42"},
		{"bool", "1", "", "true"},
		{"bool", "0", "", "false"},
		{"null", "", "", "null"},
		{"array", "", "App\\Foo", "App\\Foo[]"},
		{"array", "", "", "array[]"},
		{"object", "", "App\\Service", "App\\Service{}"},
	}

	for _, tt := range tests {
		got := formatSimpleValue(tt.typ, tt.content, tt.classname)
		if got != tt.want {
			t.Errorf("formatSimpleValue(%q, %q, %q) = %q, want %q", tt.typ, tt.content, tt.classname, got, tt.want)
		}
	}
}

func TestFormatPropertyValue(t *testing.T) {
	tests := []struct {
		name string
		prop Property
		want string
	}{
		{
			name: "base64 string",
			prop: Property{Type: "string", Encoding: "base64", Value: "SGVsbG8sIFdvcmxkIQ=="},
			want: `"Hello, World!"`,
		},
		{
			name: "int",
			prop: Property{Type: "int", Value: "42"},
			want: "42",
		},
		{
			name: "bool true",
			prop: Property{Type: "bool", Value: "1"},
			want: "true",
		},
		{
			name: "array with children",
			prop: Property{Type: "array", Children: 5},
			want: "array[5]",
		},
		{
			name: "object with classname",
			prop: Property{Type: "object", ClassName: "App\\Service", Children: 2},
			want: "App\\Service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPropertyValue(tt.prop)
			if got != tt.want {
				t.Errorf("formatPropertyValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseVariablesFromRealXML(t *testing.T) {
	data, err := os.ReadFile("testdata/context_get_complex.xml")
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}

	vars := ParseVariables(string(data))

	// Find specific variables
	findVar := func(name string) *Variable {
		for i := range vars {
			if vars[i].Name == name {
				return &vars[i]
			}
		}
		return nil
	}

	tests := []struct {
		name      string
		wantType  string
		wantValue string
	}{
		{"$count", "string", `"Hello, World!"`},
		{"$name", "string", `"World"`},
		{"$sum", "int", "15"},
		{"$numbers", "array", "array[5]"},
		{"$user", "array", "array[3]"},
		{"$obj", "object", `App\Service\MyService`},
		{"$emptyArray", "array", "array[]"},
		{"$nullVal", "null", "null"},
		{"$boolVal", "bool", "true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := findVar(tt.name)
			if v == nil {
				t.Fatalf("variable %s not found", tt.name)
			}
			if v.Type != tt.wantType {
				t.Errorf("type = %q, want %q", v.Type, tt.wantType)
			}
			if v.Value != tt.wantValue {
				t.Errorf("value = %q, want %q", v.Value, tt.wantValue)
			}
		})
	}
}
