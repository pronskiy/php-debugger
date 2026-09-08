package dbgp

import (
	"fmt"
	"testing"
)

func TestParseRealXML(t *testing.T) {
	xml := `<response xmlns="urn:debugger_protocol_v1" xmlns:xdebug="https://xdebug.org/dbgp/xdebug" command="context_get" transaction_id="5" context="0"><property name="$count" fullname="$count" type="string" size="13" encoding="base64"><![CDATA[SGVsbG8sIFdvcmxkIQ==]]></property><property name="$name" fullname="$name" type="string" size="5" encoding="base64"><![CDATA[V29ybGQ=]]></property><property name="$sum" fullname="$sum" type="int"><![CDATA[15]]></property></response>`

	vars := ParseVariables(xml)
	fmt.Printf("Found %d variables:\n", len(vars))
	for i, v := range vars {
		fmt.Printf("[%d] %s (%s) = %q\n", i, v.Name, v.Type, v.Value)
	}
	
	if len(vars) != 3 {
		t.Fatalf("expected 3 variables, got %d", len(vars))
	}
	
	if vars[0].Value != `"Hello, World!"` {
		t.Errorf("$count: expected %q, got %q", `"Hello, World!"`, vars[0].Value)
	}
}
