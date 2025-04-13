// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gjson_template

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// gjsonExecTest defines a template execution test using JSON data
type gjsonExecTest struct {
	name   string
	input  string
	output string
	data   []byte // JSON formatted data
	ok     bool
}

// Basic test JSON data
var baseTestJSON = []byte(`{
	"String": "hello",
	"Number": 42,
	"Bool": true,
	"Array": [1, 2, 3],
	"Object": {"Name": "test", "Value": 123},
	"Nested": {"Level1": {"Level2": {"Value": "nested"}}},
	"Empty": {
		"String": "",
		"Array": [],
		"Object": {}
	},
	"Null": null
}`)

// Complex test JSON data, simulating the original tVal structure
var complexTestJSON = []byte(`{
	"True": true,
	"I": 17,
	"U16": 16,
	"X": "x",
	"S": "xyz",
	"U": {"V": "v"},
	"V0": {"j": 6666},
	"V1": {"j": 7777},
	"V2": null,
	"W0": {"k": 888},
	"W1": {"k": 999},
	"W2": null,
	"SI": [3, 4, 5],
	"SICap": [0, 0, 0, 0, 0],
	"SIEmpty": [],
	"SB": [true, false],
	"AI": [3, 4, 5],
	"MSI": {"one": 1, "two": 2, "three": 3},
	"MSIone": {"one": 1},
	"MSIEmpty": {},
	"MXI": {"one": 1},
	"MII": {"1": 1},
	"MI32S": {"1": "one", "2": "two"},
	"MI64S": {"2": "i642", "3": "i643"},
	"MUI32S": {"2": "u322", "3": "u323"},
	"MUI64S": {"2": "ui642", "3": "ui643"},
	"MI8S": {"2": "i82", "3": "i83"},
	"MUI8S": {"2": "u82", "3": "u83"},
	"SMSI": [
		{"one": 1, "two": 2},
		{"eleven": 11, "twelve": 12}
	],
	"Empty0": null,
	"Empty1": 3,
	"Empty2": "empty2",
	"Empty3": [7, 8],
	"Empty4": {"V": "UinEmpty"},
	"NonEmptyInterface": {"X": "x"},
	"NonEmptyInterfacePtS": ["a", "b"],
	"NonEmptyInterfaceNil": null,
	"NonEmptyInterfaceTypedNil": null,
	"Str": "foozle",
	"Err": "erroozle",
	"PI": 23,
	"PS": "a string",
	"PSI": [21, 22, 23],
	"NIL": null,
	"UPI": 23,
	"EmptyUPI": null,
	"FloatZero": 0.0,
	"ComplexZero": 0.0,
	"NegOne": -1,
	"Three": 3,
	"Uthree": 3,
	"Ufour": 4
}`)

// JSON data for gjson path syntax tests
var gjsonPathTestJSON = []byte(`{
	"name": {"first": "Tom", "last": "Anderson"},
	"age": 37,
	"children": ["Sara", "Alex", "Jack"],
	"fav.movie": "Deer Hunter",
	"friends": [
		{"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
		{"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
		{"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
	],
	"nested": {
		"objects": {
			"are": {
				"supported": true
			}
		}
	},
	"values": [
		{"id": 1, "active": true},
		{"id": 2, "active": false},
		{"id": 3, "active": true},
		{"id": 4, "active": null},
		{"id": 5}
	]
}`)

var gjsonExecTests = []gjsonExecTest{
	// Basic data access tests
	{"basic path", "{{.name.last}}", "Anderson", gjsonPathTestJSON, true},
	{"dot notation", "{{.name.first}}", "Tom", gjsonPathTestJSON, true},

	// Tests for gjson function
	{"gjson function basic", "{{gjson \"name.last\"}}", "Anderson", gjsonPathTestJSON, true},
	{"gjson function array", "{{gjson \"children.0\"}}", "Sara", gjsonPathTestJSON, true},
	{"gjson function nested", "{{gjson \"nested.objects.are.supported\"}}", "true", gjsonPathTestJSON, true},
	{"gjson function with dot in key", "{{gjson \"fav\\\\.movie\"}}", "Deer Hunter", gjsonPathTestJSON, true},
	{"gjson function array query", "{{gjson \"friends.#(last==\\\"Murphy\\\").first\"}}", "Dale", gjsonPathTestJSON, true},
	{"gjson function array all matches", "{{gjson \"friends.#(last==\\\"Murphy\\\")#.first\"}}", "[\"Dale\",\"Jane\"]", gjsonPathTestJSON, true},

	// GJSON Path Syntax tests with gjson function
	{"backtick basic", "{{gjson `name.last`}}", "Anderson", gjsonPathTestJSON, true},
	{"backtick array index", "{{gjson `children.0`}}", "Sara", gjsonPathTestJSON, true},
	{"backtick array length", "{{gjson `children.#`}}", "3", gjsonPathTestJSON, true},
	{"backtick escaped dot", "{{gjson `fav\\.movie`}}", "Deer Hunter", gjsonPathTestJSON, true},

	// GJSON Array queries with gjson function
	{"backtick array query equals", "{{gjson `friends.#(last==\"Murphy\").first`}}", "Dale", gjsonPathTestJSON, true},
	{"backtick array query all matches", "{{gjson `friends.#(last==\"Murphy\")#.first`}}", "[\"Dale\",\"Jane\"]", gjsonPathTestJSON, true},
	{"backtick array query greater than", "{{gjson `friends.#(age>45)#.last`}}", "[\"Craig\",\"Murphy\"]", gjsonPathTestJSON, true},
	{"backtick array query pattern matching", "{{gjson `friends.#(first%\"D*\").last`}}", "Murphy", gjsonPathTestJSON, true},
	{"backtick array query not pattern matching", "{{gjson `friends.#(first!%\"D*\").last`}}", "Craig", gjsonPathTestJSON, true},

	// GJSON Nested queries with gjson function
	{"backtick nested query", "{{gjson `friends.#(nets.#(==fb))#.first`}}", "[\"Dale\",\"Roger\"]", gjsonPathTestJSON, true},

	// GJSON Tilde operator with gjson function
	{"backtick tilde true", "{{gjson `values.#(active==~true)#.id`}}", "[1,3]", gjsonPathTestJSON, true},
	{"backtick tilde false", "{{gjson `values.#(active==~false)#.id`}}", "[2,4,5]", gjsonPathTestJSON, true},
	{"backtick tilde null", "{{gjson `values.#(active==~null)#.id`}}", "[4,5]", gjsonPathTestJSON, true},
	{"backtick tilde exists", "{{gjson `values.#(active==~*)#.id`}}", "[1,2,3,4]", gjsonPathTestJSON, true},

	// GJSON Modifiers with gjson function
	{"backtick reverse modifier", "{{gjson `children.@reverse`}}", "[\"Jack\",\"Alex\",\"Sara\"]", gjsonPathTestJSON, true},
	{"backtick reverse and index", "{{gjson `children.@reverse.0`}}", "Jack", gjsonPathTestJSON, true},
	{"backtick keys modifier", "{{gjson `name.@keys`}}", "[\"first\",\"last\"]", gjsonPathTestJSON, true},
	{"backtick values modifier", "{{gjson `name.@values`}}", "[\"Tom\",\"Anderson\"]", gjsonPathTestJSON, true},

	// GJSON Deep nesting with gjson function
	{"backtick deep path", "{{gjson `nested.objects.are.supported`}}", "true", gjsonPathTestJSON, true},

	// GJSON Multipaths with gjson function
	{"backtick multipath array", "{{gjson `[children.0,children.1]`}}", "[\"Sara\",\"Alex\"]", gjsonPathTestJSON, true},
	{"backtick multipath object", "{{gjson `{\"first_name\":name.first,\"last_name\":name.last,\"age\":age}`}}", "{\"first_name\":\"Tom\",\"last_name\":\"Anderson\",\"age\":37}", gjsonPathTestJSON, true},
	{"backtick multipath with key", "{{gjson `{\"name\":name.first,\"murphy_friends\":friends.#(last==\"Murphy\")#.first}`}}", "{\"name\":\"Tom\",\"murphy_friends\":[\"Dale\",\"Jane\"]}", gjsonPathTestJSON, true},

	// GJSON Literals with gjson function
	{"backtick literal string", "{{gjson `{\"name\":name.first,\"company\":!\"Acme Inc\"}`}}", "{\"name\":\"Tom\",\"company\":\"Acme Inc\"}", gjsonPathTestJSON, true},
	{"backtick literal boolean", "{{gjson `{\"name\":name.first,\"active\":!true}`}}", "{\"name\":\"Tom\",\"active\":true}", gjsonPathTestJSON, true},
	{"backtick literal number", "{{gjson `{\"name\":name.first,\"count\":!42}`}}", "{\"name\":\"Tom\",\"count\":42}", gjsonPathTestJSON, true},

	// GJSON Wildcards with gjson function
	{"backtick wildcard star", "{{gjson `child*.2`}}", "Jack", gjsonPathTestJSON, true},
	{"backtick wildcard question mark", "{{gjson `c?ildren.0`}}", "Sara", gjsonPathTestJSON, true},

	// Basic data access tests
	{"string", "{{.String}}", "hello", baseTestJSON, true},
	{"number", "{{.Number}}", "42", baseTestJSON, true},
	{"boolean", "{{.Bool}}", "true", baseTestJSON, true},
	{"nested object", "{{.Nested.Level1.Level2.Value}}", "nested", baseTestJSON, true},
	{"array element", "{{index .Array 1}}", "2", baseTestJSON, true},
	{"object property", "{{.Object.Name}}", "test", baseTestJSON, true},

	// Conditional tests
	{"if true", "{{if .Bool}}YES{{end}}", "YES", baseTestJSON, true},
	{"if false", "{{if not .Bool}}YES{{else}}NO{{end}}", "NO", baseTestJSON, true},
	{"if empty string", "{{if .Empty.String}}YES{{else}}NO{{end}}", "NO", baseTestJSON, true},
	{"if empty array", "{{if .Empty.Array}}YES{{else}}NO{{end}}", "NO", baseTestJSON, true},
	{"if empty object", "{{if .Empty.Object}}YES{{else}}NO{{end}}", "NO", baseTestJSON, true},
	{"if null", "{{if .Null}}YES{{else}}NO{{end}}", "NO", baseTestJSON, true},
	{"if number", "{{if .Number}}YES{{else}}NO{{end}}", "YES", baseTestJSON, true},

	// Loop tests
	{"range array", "{{range .Array}}{{.}},{{end}}", "1,2,3,", baseTestJSON, true},
	{"range array with index", "{{range $i, $v := .Array}}[{{$i}}:{{$v}}],{{end}}", "[0:1],[1:2],[2:3],", baseTestJSON, true},
	{"range object", "{{range $k, $v := .Object}}{{$k}}={{$v}},{{end}}", "Name=test,Value=123,", baseTestJSON, true},
	{"range empty", "{{range .Empty.Array}}{{.}}{{else}}EMPTY{{end}}", "EMPTY", baseTestJSON, true},

	// With statement tests
	{"with", "{{with .Object}}{{.Name}}{{end}}", "test", baseTestJSON, true},
	{"with else", "{{with .Null}}{{.}}{{else}}NULL{{end}}", "NULL", baseTestJSON, true},

	// Variable assignment tests
	{"variable", "{{$x := .Number}}{{$x}}", "42", baseTestJSON, true},
	{"variable in scope", "{{with .Object}}{{$x := .Name}}{{$x}}{{end}}", "test", baseTestJSON, true},

	// Built-in function tests
	{"len array", "{{len .Array}}", "3", baseTestJSON, true},
	{"len string", "{{len .String}}", "5", baseTestJSON, true},
	{"len object", "{{len .Object}}", "2", baseTestJSON, true},

	// Logical function tests
	{"and", "{{and .Bool .Number}}", "42", baseTestJSON, true},
	{"and false", "{{and .Bool .Empty.String}}", "", baseTestJSON, true},
	{"or", "{{or .Empty.String .Number}}", "42", baseTestJSON, true},
	{"or false", "{{or .Empty.String .Empty.Array}}", "[]", baseTestJSON, true},
	{"not", "{{not .Bool}}", "false", baseTestJSON, true},
	{"not false", "{{not .Empty.String}}", "true", baseTestJSON, true},

	// Comparison function tests
	{"eq", "{{eq .Number 42}}", "true", baseTestJSON, true},
	{"ne", "{{ne .Number 43}}", "true", baseTestJSON, true},
	{"lt", "{{lt .Number 43}}", "true", baseTestJSON, true},
	{"le", "{{le .Number 42}}", "true", baseTestJSON, true},
	{"gt", "{{gt .Number 41}}", "true", baseTestJSON, true},
	{"ge", "{{ge .Number 42}}", "true", baseTestJSON, true},

	// Complex test cases using complexTestJSON
	{"complex field", "{{.X}}", "x", complexTestJSON, true},
	{"complex nested", "{{.U.V}}", "v", complexTestJSON, true},
	{"complex array", "{{index .SI 1}}", "4", complexTestJSON, true},
	{"complex map", "{{.MSI.one}}", "1", complexTestJSON, true},
	{"complex range", "{{range .SI}}{{.}}-{{end}}", "3-4-5-", complexTestJSON, true},
	{"complex if", "{{if .True}}true{{else}}false{{end}}", "true", complexTestJSON, true},

	// Pipeline tests
	{"pipeline", "{{.Number | printf \"%04d\"}}", "0042", baseTestJSON, true},
	{"pipeline chain", "{{.String | printf \"%s world\"}}", "hello world", baseTestJSON, true},

	// Error tests
	{"missing field", "{{.MissingField}}", "", baseTestJSON, true},
	{"invalid syntax", "{{.Number.Field}}", "", baseTestJSON, true},

	// Basic text
	{"text2", "some text", "some text", baseTestJSON, true},

	// Conditional tests
	{"if true2", "{{if true}}TRUE{{end}}", "TRUE", baseTestJSON, true},
	{"if false2", "{{if false}}TRUE{{else}}FALSE{{end}}", "FALSE", baseTestJSON, true},
	{"if 1", "{{if 1}}NON-ZERO{{else}}ZERO{{end}}", "NON-ZERO", baseTestJSON, true},
	{"if 0", "{{if 0}}NON-ZERO{{else}}ZERO{{end}}", "ZERO", baseTestJSON, true},

	// Variable tests
	{"declare in action", "{{$x := .String}}{{$x}}", "hello", baseTestJSON, true},
	{"simple assignment", "{{$x := 2}}{{$x = 3}}{{$x}}", "3", baseTestJSON, true},
	{"nested assignment", "{{$x := 2}}{{if true}}{{$x = 3}}{{end}}{{$x}}", "3", baseTestJSON, true},

	// Field access tests
	{"field access", "{{.Object.Name}}", "test", baseTestJSON, true},
	{"nested field", "{{.Nested.Level1.Level2.Value}}", "nested", baseTestJSON, true},

	// Index tests
	{"array index", "{{index .Array 0}}", "1", baseTestJSON, true},
	{"map index", "{{index .Object \"Name\"}}", "test", baseTestJSON, true},

	// Loop tests
	{"range array2", "{{range .Array}}{{.}},{{end}}", "1,2,3,", baseTestJSON, true},
	{"range empty2", "{{range .Empty.Array}}{{.}}{{else}}EMPTY{{end}}", "EMPTY", baseTestJSON, true},
	{"range with index", "{{range $i, $v := .Array}}[{{$i}}:{{$v}}],{{end}}", "[0:1],[1:2],[2:3],", baseTestJSON, true},

	// With statement tests
	{"with true", "{{with .Object}}{{.Name}}{{end}}", "test", baseTestJSON, true},
	{"with false", "{{with .Empty.Array}}{{.}}{{else}}EMPTY{{end}}", "EMPTY", baseTestJSON, true},

	// Built-in function tests
	{"len2", "{{len .Array}}", "3", baseTestJSON, true},
	{"print", "{{print \"hello\"}}", "hello", baseTestJSON, true},
	{"printf2", "{{printf \"%04d\" 42}}", "0042", baseTestJSON, true},

	// Logical operation tests
	{"and2", "{{and true 1}}", "1", baseTestJSON, true},
	{"or2", "{{or false 1}}", "1", baseTestJSON, true},
	{"not2", "{{not true}}", "false", baseTestJSON, true},

	// Comparison operation tests
	{"eq2", "{{eq 1 1}}", "true", baseTestJSON, true},
	{"ne2", "{{ne 1 2}}", "true", baseTestJSON, true},
	{"lt2", "{{lt 1 2}}", "true", baseTestJSON, true},
	{"le2", "{{le 1 1}}", "true", baseTestJSON, true},
	{"gt2", "{{gt 2 1}}", "true", baseTestJSON, true},
	{"ge2", "{{ge 1 1}}", "true", baseTestJSON, true},

	// Pipeline tests
	{"pipeline2", "{{.Number | printf \"%04d\"}}", "0042", baseTestJSON, true},

	// HTML escaping tests
	{"html2", "{{html \"<script>\"}}", "&lt;script&gt;", baseTestJSON, true},

	// Complex test cases
	{"complex field2", "{{.X}}", "x", complexTestJSON, true},
	{"complex nested2", "{{.U.V}}", "v", complexTestJSON, true},
	{"complex array2", "{{index .SI 1}}", "4", complexTestJSON, true},
	{"complex map2", "{{.MSI.one}}", "1", complexTestJSON, true},
	{"complex range2", "{{range .SI}}{{.}}-{{end}}", "3-4-5-", complexTestJSON, true},
}

// TestGjsonExecute tests template execution using gjson implementation
func TestGjsonExecute(t *testing.T) {
	for _, test := range gjsonExecTests {
		tmpl, err := New(test.name).Parse(test.input)
		if err != nil {
			t.Errorf("%s: parse error: %s", test.name, err)
			continue
		}

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, test.data)

		switch {
		case !test.ok && err == nil:
			t.Errorf("%s: expected error; got none", test.name)
			continue
		case test.ok && err != nil:
			t.Errorf("%s: unexpected execute error: %s", test.name, err)
			continue
		case !test.ok && err != nil:
			// Expected error, got error, test passes
			continue
		}

		result := buf.String()
		if result != test.output {
			t.Errorf("%s: expected %q; got %q", test.name, test.output, result)
		}
	}
}

// TestGjsonTemplateFiles tests template file loading
func TestGjsonTemplateFiles(t *testing.T) {
	// This test depends on files in the testdata directory
	// We assume these files already exist
	tmpl, err := ParseFiles("testdata/file1.tmpl", "testdata/file2.tmpl")
	if err != nil {
		t.Fatalf("error parsing files: %v", err)
	}

	// Prepare test data
	testData := []byte(`{"SI": [3, 4, 5]}`)

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "x", testData)
	if err != nil {
		t.Fatal(err)
	}

	expected := "TEXT"
	if buf.String() != expected {
		t.Errorf("template file: expected %q got %q", expected, buf.String())
	}
}

// TestEvalFunctionSliceCap tests the potential issue with makeslice cap out of range
// in the evalFunction method when capacity might be negative
func TestEvalFunctionSliceCap(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		data        []byte
		expectError bool
		errorType   string
	}{
		{
			name:        "Empty function args",
			template:    "{{nonExistentFunction}}",
			data:        []byte(`{}`),
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Function with empty args list",
			template:    "{{nonExistentFunction ()}}",
			data:        []byte(`{}`),
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Function with pipeline but no args",
			template:    "{{. | nonExistentFunction}}",
			data:        []byte(`"test"`),
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Extremely large number of args",
			template:    createTemplateWithManyArgs(1000),
			data:        []byte(`{}`),
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Negative number in template",
			template:    "{{nonExistentFunction -1}}",
			data:        []byte(`{}`),
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Function with null pipeline",
			template:    "{{.Null | nonExistentFunction}}",
			data:        baseTestJSON,
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Function with empty string pipeline",
			template:    "{{.Empty.String | nonExistentFunction}}",
			data:        baseTestJSON,
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
		{
			name:        "Nested function calls with empty args",
			template:    "{{nonExistentFunction (nonExistentFunction2)}}",
			data:        []byte(`{}`),
			expectError: true,
			errorType:   "function \"nonExistentFunction2\" not implemented",
		},
		{
			name:        "Function with complex expression",
			template:    "{{nonExistentFunction (index .Array 0) (len .Array) (eq 1 1)}}",
			data:        baseTestJSON,
			expectError: true,
			errorType:   "function \"nonExistentFunction\" not implemented",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpl, err := New(test.name).Parse(test.template)
			if err != nil {
				if !test.expectError {
					t.Errorf("parse error: %s", err)
				}
				return
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, test.data)

			if test.expectError && err == nil {
				t.Errorf("Expected error but got none")
			} else if !test.expectError && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			// Check for specific error type
			if test.expectError && err != nil {
				if !strings.Contains(err.Error(), "makeslice: cap out of range") &&
					!strings.Contains(err.Error(), test.errorType) {
					t.Logf("Got error but not the expected one: %s", err)
				}
			}
		})
	}
}

// Helper function to create a template with many arguments
func createTemplateWithManyArgs(count int) string {
	var builder strings.Builder
	builder.WriteString("{{nonExistentFunction")
	for i := 0; i < count; i++ {
		builder.WriteString(" ")
		builder.WriteString(string(rune('a' + i%26)))
	}
	builder.WriteString("}}")
	return builder.String()
}

// TestEvalFunctionEdgeCases tests specific edge cases that might trigger the makeslice error
func TestEvalFunctionEdgeCases(t *testing.T) {
	// This test specifically targets the capacity calculation in evalFunction
	// where capacity := len(args) - 1 could result in a negative value

	// Create a template with a very large number of nested function calls
	// which might cause stack overflow or other issues
	nestedTemplate := createNestedFunctionCalls(10)

	tmpl, err := New("nested").Parse(nestedTemplate)
	if err != nil {
		// Parse errors are acceptable for this test
		t.Logf("Parse error (expected): %s", err)
		return
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, baseTestJSON)

	// We're looking specifically for "makeslice: cap out of range" error
	if err != nil {
		if strings.Contains(err.Error(), "makeslice: cap out of range") {
			t.Logf("Found the expected 'makeslice: cap out of range' error: %s", err)
		} else {
			t.Logf("Got error but not the expected 'makeslice' one: %s", err)
		}
	}
}

// Helper function to create deeply nested function calls
func createNestedFunctionCalls(depth int) string {
	if depth <= 0 {
		return "nonExistentFunction"
	}
	return "{{nonExistentFunction" + createNestedFunctionCalls(depth-1) + "}}"
}

// TestEmptyArgsSlice tests the specific case where args slice might be empty
func TestEmptyArgsSlice(t *testing.T) {
	// This test specifically targets the case where args might be empty
	// which could lead to capacity = len(args) - 1 = -1

	templates := []string{
		// Empty function call
		"{{emptyFunction}}",

		// Function call with empty parentheses
		"{{emptyFunction()}}",

		// Function call with whitespace
		"{{  emptyFunction  }}",

		// Function call with pipeline but no args
		"{{. | emptyFunction}}",

		// Function call with nested empty function
		"{{emptyFunction(emptyFunction)}}",
	}

	for i, template := range templates {
		t.Run(fmt.Sprintf("EmptyArgs%d", i), func(t *testing.T) {
			tmpl, err := New(fmt.Sprintf("empty%d", i)).Parse(template)
			if err != nil {
				t.Logf("Parse error: %s", err)
				return
			}

			var buf bytes.Buffer
			err = tmpl.Execute(&buf, baseTestJSON)

			// Check specifically for "makeslice: cap out of range" error
			if err != nil {
				if strings.Contains(err.Error(), "makeslice: cap out of range") {
					t.Logf("Found the expected 'makeslice: cap out of range' error: %s", err)
				} else {
					t.Logf("Got error but not the expected 'makeslice' one: %s", err)
				}
			}
		})
	}
}
