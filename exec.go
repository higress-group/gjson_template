// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package gjson_template

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"runtime"
	"strconv"
	"strings"

	"github.com/higress-group/gjson_template/parse"

	"github.com/tidwall/gjson"
)

// maxExecDepth specifies the maximum stack depth of templates within
// templates. This limit is only practically reached by accidentally
// recursive template invocations. This limit allows us to return
// an error instead of triggering a stack overflow.
var maxExecDepth = initMaxExecDepth()

func initMaxExecDepth() int {
	if runtime.GOARCH == "wasm" {
		return 1000
	}
	return 100000
}

// state represents the state of an execution. It's not part of the
// template so that multiple executions of the same template
// can execute in parallel.
type state struct {
	tmpl       *Template
	wr         io.Writer
	node       parse.Node   // current node, for errors
	vars       []variable   // push-down stack of variable values.
	depth      int          // the height of the stack of executing templates.
	jsonData   gjson.Result // root JSON data
	strictMode bool         // whether to error on missing paths
}

// variable holds the dynamic value of a variable such as $, $x etc.
type variable struct {
	name  string
	value gjson.Result
}

// push pushes a new variable on the stack.
func (s *state) push(name string, value gjson.Result) {
	s.vars = append(s.vars, variable{name, value})
}

// mark returns the length of the variable stack.
func (s *state) mark() int {
	return len(s.vars)
}

// pop pops the variable stack up to the mark.
func (s *state) pop(mark int) {
	s.vars = s.vars[0:mark]
}

// setVar overwrites the last declared variable with the given name.
// Used by variable assignments.
func (s *state) setVar(name string, value gjson.Result) {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			s.vars[i].value = value
			return
		}
	}
	s.errorf("undefined variable: %s", name)
}

// setTopVar overwrites the top-nth variable on the stack. Used by range iterations.
func (s *state) setTopVar(n int, value gjson.Result) {
	s.vars[len(s.vars)-n].value = value
}

// varValue returns the value of the named variable.
func (s *state) varValue(name string) gjson.Result {
	for i := s.mark() - 1; i >= 0; i-- {
		if s.vars[i].name == name {
			return s.vars[i].value
		}
	}
	s.errorf("undefined variable: %s", name)
	return gjson.Result{}
}

var zero reflect.Value

type missingValType struct{}

var missingVal = reflect.ValueOf(missingValType{})

var missingValReflectType = reflect.TypeFor[missingValType]()

func isMissing(v reflect.Value) bool {
	return v.IsValid() && v.Type() == missingValReflectType
}

// at marks the state to be on node n, for error reporting.
func (s *state) at(node parse.Node) {
	s.node = node
}

// doublePercent returns the string with %'s replaced by %%, if necessary,
// so it can be used safely inside a Printf format string.
func doublePercent(str string) string {
	return strings.ReplaceAll(str, "%", "%%")
}

// TODO: It would be nice if ExecError was more broken down, but
// the way ErrorContext embeds the template name makes the
// processing too clumsy.

// ExecError is the custom error type returned when Execute has an
// error evaluating its template. (If a write error occurs, the actual
// error is returned; it will not be of type ExecError.)
type ExecError struct {
	Name string // Name of template.
	Err  error  // Pre-formatted error.
}

func (e ExecError) Error() string {
	return e.Err.Error()
}

func (e ExecError) Unwrap() error {
	return e.Err
}

// errorf records an ExecError and terminates processing.
func (s *state) errorf(format string, args ...any) {
	name := doublePercent(s.tmpl.Name())
	if s.node == nil {
		format = fmt.Sprintf("template: %s: %s", name, format)
	} else {
		location, context := s.tmpl.ErrorContext(s.node)
		format = fmt.Sprintf("template: %s: executing %q at <%s>: %s", location, name, doublePercent(context), format)
	}
	panic(ExecError{
		Name: s.tmpl.Name(),
		Err:  fmt.Errorf(format, args...),
	})
}

// writeError is the wrapper type used internally when Execute has an
// error writing to its output. We strip the wrapper in errRecover.
// Note that this is not an implementation of error, so it cannot escape
// from the package as an error value.
type writeError struct {
	Err error // Original error.
}

func (s *state) writeError(err error) {
	panic(writeError{
		Err: err,
	})
}

// errRecover is the handler that turns panics into returns from the top
// level of Parse.
func errRecover(errp *error) {
	e := recover()
	if e != nil {
		switch err := e.(type) {
		case runtime.Error:
			panic(e)
		case writeError:
			*errp = err.Err // Strip the wrapper.
		case ExecError:
			*errp = err // Keep the wrapper.
		default:
			panic(e)
		}
	}
}

// ExecuteTemplate applies the template associated with t that has the given name
// to the specified JSON data and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) ExecuteTemplate(wr io.Writer, name string, data []byte) error {
	tmpl := t.Lookup(name)
	if tmpl == nil {
		return fmt.Errorf("template: no template %q associated with template %q", name, t.name)
	}
	return tmpl.Execute(wr, data)
}

// Execute applies a parsed template to the specified JSON data,
// and writes the output to wr.
// If an error occurs executing the template or writing its output,
// execution stops, but partial results may already have been written to
// the output writer.
// A template may be executed safely in parallel, although if parallel
// executions share a Writer the output may be interleaved.
func (t *Template) Execute(wr io.Writer, data []byte) error {
	return t.execute(wr, data)
}

func (t *Template) execute(wr io.Writer, data []byte) (err error) {
	defer errRecover(&err)

	// Parse JSON data
	jsonResult := gjson.ParseBytes(data)
	if !jsonResult.IsObject() && !jsonResult.IsArray() {
		return fmt.Errorf("template: %s: data must be a valid JSON object or array", t.Name())
	}

	state := &state{
		tmpl:       t,
		wr:         wr,
		jsonData:   jsonResult,
		vars:       []variable{{"$", jsonResult}},
		strictMode: false, // Default to non-strict mode
	}

	if t.Tree == nil || t.Root == nil {
		state.errorf("%q is an incomplete or empty template", t.Name())
	}

	state.walk(jsonResult, t.Root)
	return
}

// DefinedTemplates returns a string listing the defined templates,
// prefixed by the string "; defined templates are: ". If there are none,
// it returns the empty string. For generating an error message here
// and in [html/template].
func (t *Template) DefinedTemplates() string {
	if t.common == nil {
		return ""
	}
	var b strings.Builder
	t.muTmpl.RLock()
	defer t.muTmpl.RUnlock()
	for name, tmpl := range t.tmpl {
		if tmpl.Tree == nil || tmpl.Root == nil {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("; defined templates are: ")
		} else {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%q", name)
	}
	return b.String()
}

// Sentinel errors for use with panic to signal early exits from range loops.
var (
	walkBreak    = errors.New("break")
	walkContinue = errors.New("continue")
)

// Walk functions step through the major pieces of the template structure,
// generating output as they go.
func (s *state) walk(dot gjson.Result, node parse.Node) {
	s.at(node)
	switch node := node.(type) {
	case *parse.ActionNode:
		// Do not pop variables so they persist until next end.
		// Also, if the action declares variables, don't print the result.
		val := s.evalPipeline(dot, node.Pipe)
		if len(node.Pipe.Decl) == 0 {
			s.printValue(node, val)
		}
	case *parse.BreakNode:
		panic(walkBreak)
	case *parse.CommentNode:
	case *parse.ContinueNode:
		panic(walkContinue)
	case *parse.IfNode:
		s.walkIfOrWith(parse.NodeIf, dot, node.Pipe, node.List, node.ElseList)
	case *parse.ListNode:
		for _, node := range node.Nodes {
			s.walk(dot, node)
		}
	case *parse.RangeNode:
		s.walkRange(dot, node)
	case *parse.TemplateNode:
		s.walkTemplate(dot, node)
	case *parse.TextNode:
		if _, err := s.wr.Write(node.Text); err != nil {
			s.writeError(err)
		}
	case *parse.WithNode:
		s.walkIfOrWith(parse.NodeWith, dot, node.Pipe, node.List, node.ElseList)
	default:
		s.errorf("unknown node: %s", node)
	}
}

// walkIfOrWith walks an 'if' or 'with' node. The two control structures
// are identical in behavior except that 'with' sets dot.
func (s *state) walkIfOrWith(typ parse.NodeType, dot gjson.Result, pipe *parse.PipeNode, list, elseList *parse.ListNode) {
	defer s.pop(s.mark())
	val := s.evalPipeline(dot, pipe)
	truth, ok := isGjsonTrue(val)
	if !ok {
		s.errorf("if/with can't use %v", val)
	}
	if truth {
		if typ == parse.NodeWith {
			s.walk(val, list)
		} else {
			s.walk(dot, list)
		}
	} else if elseList != nil {
		s.walk(dot, elseList)
	}
}

// isGjsonTrue reports whether the gjson.Result value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value.
func isGjsonTrue(val gjson.Result) (truth, ok bool) {
	if !val.Exists() {
		return false, true
	}

	switch val.Type {
	case gjson.Null:
		return false, true
	case gjson.False:
		return false, true
	case gjson.True:
		return true, true
	case gjson.Number:
		return val.Num != 0, true
	case gjson.String:
		return val.Str != "", true
	case gjson.JSON:
		if val.IsArray() || val.IsObject() {
			return val.Raw != "" && val.Raw != "[]" && val.Raw != "{}", true
		}
	}
	return false, false
}

// IsTrue reports whether the value is 'true', in the sense of not the zero of its type,
// and whether the value has a meaningful truth value. This is the definition of
// truth used by if and other such actions.
func IsTrue(val any) (truth, ok bool) {
	return isTrue(reflect.ValueOf(val))
}

func isTrue(val reflect.Value) (truth, ok bool) {
	if !val.IsValid() {
		// Something like var x interface{}, never set. It's a form of nil.
		return false, true
	}
	switch val.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		truth = val.Len() > 0
	case reflect.Bool:
		truth = val.Bool()
	case reflect.Complex64, reflect.Complex128:
		truth = val.Complex() != 0
	case reflect.Chan, reflect.Func, reflect.Pointer, reflect.UnsafePointer, reflect.Interface:
		truth = !val.IsNil()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		truth = val.Int() != 0
	case reflect.Float32, reflect.Float64:
		truth = val.Float() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		truth = val.Uint() != 0
	case reflect.Struct:
		truth = true // Struct values are always true.
	default:
		return
	}
	return truth, true
}

func (s *state) walkRange(dot gjson.Result, r *parse.RangeNode) {
	s.at(r)
	defer func() {
		if r := recover(); r != nil && r != walkBreak {
			panic(r)
		}
	}()
	defer s.pop(s.mark())
	val := s.evalPipeline(dot, r.Pipe)
	// mark top of stack before any variables in the body are pushed.
	mark := s.mark()
	oneIteration := func(index, elem gjson.Result) {
		if len(r.Pipe.Decl) > 0 {
			if r.Pipe.IsAssign {
				// With two variables, index comes first.
				// With one, we use the element.
				if len(r.Pipe.Decl) > 1 {
					s.setVar(r.Pipe.Decl[0].Ident[0], index)
				} else {
					s.setVar(r.Pipe.Decl[0].Ident[0], elem)
				}
			} else {
				// Set top var (lexically the second if there
				// are two) to the element.
				s.setTopVar(1, elem)
			}
		}
		if len(r.Pipe.Decl) > 1 {
			if r.Pipe.IsAssign {
				s.setVar(r.Pipe.Decl[1].Ident[0], elem)
			} else {
				// Set next var (lexically the first if there
				// are two) to the index.
				s.setTopVar(2, index)
			}
		}
		defer s.pop(mark)
		defer func() {
			// Consume panic(walkContinue)
			if r := recover(); r != nil && r != walkContinue {
				panic(r)
			}
		}()
		s.walk(elem, r.List)
	}

	// Handle array/slice iteration
	if val.IsArray() {
		if val.Array() == nil || len(val.Array()) == 0 {
			if r.ElseList != nil {
				s.walk(dot, r.ElseList)
			}
			return
		}

		for i, elem := range val.Array() {
			indexResult := gjson.Parse(fmt.Sprintf("%d", i))
			oneIteration(indexResult, elem)
		}
		return
	}

	// Handle object/map iteration
	if val.IsObject() {
		if !val.Exists() || val.Raw == "{}" {
			if r.ElseList != nil {
				s.walk(dot, r.ElseList)
			}
			return
		}

		val.ForEach(func(key, value gjson.Result) bool {
			oneIteration(key, value)
			return true // continue iteration
		})
		return
	}

	// Handle primitive types (numbers, strings, etc.)
	if val.Type == gjson.Number {
		if len(r.Pipe.Decl) > 1 {
			s.errorf("can't use %v to iterate over more than one variable", val.Raw)
		}

		num := int(val.Int())
		if num <= 0 {
			if r.ElseList != nil {
				s.walk(dot, r.ElseList)
			}
			return
		}

		for i := 0; i < num; i++ {
			indexResult := gjson.Parse(fmt.Sprintf("%d", i))
			valueResult := gjson.Parse(fmt.Sprintf("%d", i))
			oneIteration(indexResult, valueResult)
		}
		return
	}

	// Handle string iteration
	if val.Type == gjson.String {
		str := val.String()
		if str == "" {
			if r.ElseList != nil {
				s.walk(dot, r.ElseList)
			}
			return
		}

		for i, r := range str {
			indexResult := gjson.Parse(fmt.Sprintf("%d", i))
			valueResult := gjson.Parse(fmt.Sprintf("%q", string(r)))
			oneIteration(indexResult, valueResult)
		}
		return
	}

	// Default case - can't iterate
	s.errorf("range can't iterate over %v", val.Raw)

	if r.ElseList != nil {
		s.walk(dot, r.ElseList)
	}
}

func (s *state) walkTemplate(dot gjson.Result, t *parse.TemplateNode) {
	s.at(t)
	tmpl := s.tmpl.Lookup(t.Name)
	if tmpl == nil {
		s.errorf("template %q not defined", t.Name)
	}
	if s.depth == maxExecDepth {
		s.errorf("exceeded maximum template depth (%v)", maxExecDepth)
	}
	// Variables declared by the pipeline persist.
	dot = s.evalPipeline(dot, t.Pipe)
	newState := *s
	newState.depth++
	newState.tmpl = tmpl
	// No dynamic scoping: template invocations inherit no variables.
	newState.vars = []variable{{"$", dot}}
	newState.walk(dot, tmpl.Root)
}

// Eval functions evaluate pipelines, commands, and their elements and extract
// values from the data structure by examining fields, calling methods, and so on.
// The printing of those values happens only through walk functions.

// evalPipeline returns the value acquired by evaluating a pipeline. If the
// pipeline has a variable declaration, the variable will be pushed on the
// stack. Callers should therefore pop the stack after they are finished
// executing commands depending on the pipeline value.
func (s *state) evalPipeline(dot gjson.Result, pipe *parse.PipeNode) (value gjson.Result) {
	if pipe == nil {
		return gjson.Result{}
	}
	s.at(pipe)
	value = gjson.Result{}
	for _, cmd := range pipe.Cmds {
		value = s.evalCommand(dot, cmd, value) // previous value is this one's final arg.
	}
	for _, variable := range pipe.Decl {
		if pipe.IsAssign {
			s.setVar(variable.Ident[0], value)
		} else {
			s.push(variable.Ident[0], value)
		}
	}
	return value
}

func (s *state) notAFunction(args []parse.Node, final gjson.Result) {
	if len(args) > 1 || final.Exists() {
		s.errorf("can't give argument to non-function %s", args[0])
	}
}

func (s *state) evalCommand(dot gjson.Result, cmd *parse.CommandNode, final gjson.Result) gjson.Result {
	firstWord := cmd.Args[0]

	// Check if firstWord is a StringNode with backticks, indicating gjson path syntax
	if strNode, ok := firstWord.(*parse.StringNode); ok && strings.HasPrefix(strNode.Text, "`") && strings.HasSuffix(strNode.Text, "`") {
		// Extract the gjson path from between the backticks
		path := strNode.Text[1 : len(strNode.Text)-1]

		// Use gjson's Get method directly with the extracted path
		result := dot.Get(path)

		// Check if the result exists
		if !result.Exists() && s.tmpl.option.missingKey == mapError {
			s.errorf("gjson path %q not found in data", path)
		}

		// Check if there are arguments (method call)
		if len(cmd.Args) > 1 || final.Exists() {
			s.errorf("gjson path %q is not a method but has arguments", path)
		}

		return result
	}

	// If not using gjson path syntax, proceed with normal command evaluation
	switch n := firstWord.(type) {
	case *parse.FieldNode:
		return s.evalFieldNode(dot, n, cmd.Args, final)
	case *parse.ChainNode:
		return s.evalChainNode(dot, n, cmd.Args, final)
	case *parse.IdentifierNode:
		// Must be a function.
		return s.evalFunction(dot, n, cmd, cmd.Args, final)
	case *parse.PipeNode:
		// Parenthesized pipeline. The arguments are all inside the pipeline; final must be absent.
		s.notAFunction(cmd.Args, final)
		return s.evalPipeline(dot, n)
	case *parse.VariableNode:
		return s.evalVariableNode(dot, n, cmd.Args, final)
	}
	s.at(firstWord)
	s.notAFunction(cmd.Args, final)
	switch word := firstWord.(type) {
	case *parse.BoolNode:
		return gjson.Parse(fmt.Sprintf("%t", word.True))
	case *parse.DotNode:
		return dot
	case *parse.NilNode:
		s.errorf("nil is not a command")
	case *parse.NumberNode:
		return s.idealConstantGjson(word)
	case *parse.StringNode:
		return gjson.Parse(fmt.Sprintf("%q", word.Text))
	}
	s.errorf("can't evaluate command %q", firstWord)
	panic("not reached")
}

// idealConstantGjson is called to return the gjson.Result value of a number
func (s *state) idealConstantGjson(constant *parse.NumberNode) gjson.Result {
	s.at(constant)
	switch {
	case constant.IsComplex:
		// JSON doesn't support complex numbers, so we'll convert to string
		return gjson.Parse(fmt.Sprintf("%q", fmt.Sprintf("%v", constant.Complex128)))
	case constant.IsFloat:
		// For integers represented as float, return as integer
		if constant.Float64 == float64(int64(constant.Float64)) {
			return gjson.Parse(fmt.Sprintf("%d", int64(constant.Float64)))
		}
		return gjson.Parse(fmt.Sprintf("%f", constant.Float64))
	case constant.IsInt:
		return gjson.Parse(fmt.Sprintf("%d", constant.Int64))
	case constant.IsUint:
		return gjson.Parse(fmt.Sprintf("%d", constant.Uint64))
	}
	return gjson.Result{}
}

// idealConstant is called to return the value of a number in a context where
// we don't know the type. In that case, the syntax of the number tells us
// its type, and we use Go rules to resolve. Note there is no such thing as
// a uint ideal constant in this situation - the value must be of int type.
func (s *state) idealConstant(constant *parse.NumberNode) reflect.Value {
	// These are ideal constants but we don't know the type
	// and we have no context.  (If it was a method argument,
	// we'd know what we need.) The syntax guides us to some extent.
	s.at(constant)
	switch {
	case constant.IsComplex:
		return reflect.ValueOf(constant.Complex128) // incontrovertible.

	case constant.IsFloat &&
		!isHexInt(constant.Text) && !isRuneInt(constant.Text) &&
		strings.ContainsAny(constant.Text, ".eEpP"):
		return reflect.ValueOf(constant.Float64)

	case constant.IsInt:
		n := int(constant.Int64)
		if int64(n) != constant.Int64 {
			s.errorf("%s overflows int", constant.Text)
		}
		return reflect.ValueOf(n)

	case constant.IsUint:
		s.errorf("%s overflows int", constant.Text)
	}
	return zero
}

func isRuneInt(s string) bool {
	return len(s) > 0 && s[0] == '\''
}

func isHexInt(s string) bool {
	return len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') && !strings.ContainsAny(s, "pP")
}

func (s *state) evalFieldNode(dot gjson.Result, field *parse.FieldNode, args []parse.Node, final gjson.Result) gjson.Result {
	s.at(field)
	return s.evalFieldChain(dot, dot, field, field.Ident, args, final)
}

func (s *state) evalChainNode(dot gjson.Result, chain *parse.ChainNode, args []parse.Node, final gjson.Result) gjson.Result {
	s.at(chain)
	if len(chain.Field) == 0 {
		s.errorf("internal error: no fields in evalChainNode")
	}
	if chain.Node.Type() == parse.NodeNil {
		s.errorf("indirection through explicit nil in %s", chain)
	}
	// (pipe).Field1.Field2 has pipe as .Node, fields as .Field. Eval the pipeline, then the fields.
	pipe := s.evalArg(dot, chain.Node)
	return s.evalFieldChain(dot, pipe, chain, chain.Field, args, final)
}

func (s *state) evalVariableNode(dot gjson.Result, variable *parse.VariableNode, args []parse.Node, final gjson.Result) gjson.Result {
	// $x.Field has $x as the first ident, Field as the second. Eval the var, then the fields.
	s.at(variable)
	value := s.varValue(variable.Ident[0])
	if len(variable.Ident) == 1 {
		s.notAFunction(args, final)
		return value
	}
	return s.evalFieldChain(dot, value, variable, variable.Ident[1:], args, final)
}

// evalFieldChain evaluates .X.Y.Z possibly followed by arguments.
// dot is the environment in which to evaluate arguments, while
// receiver is the value being walked along the chain.
func (s *state) evalFieldChain(dot, receiver gjson.Result, node parse.Node, ident []string, args []parse.Node, final gjson.Result) gjson.Result {
	// Build a gjson path from the identifiers
	path := strings.Join(ident, ".")

	// Use gjson's native Get method to retrieve the value
	result := receiver.Get(path)

	// Check if the result exists
	if !result.Exists() && s.tmpl.option.missingKey == mapError {
		s.errorf("path %q not found in data", path)
	}

	// Check if there are arguments (method call)
	if len(args) > 1 || final.Exists() {
		s.errorf("%s is not a method but has arguments", path)
	}

	return result
}

func (s *state) evalFunction(dot gjson.Result, node *parse.IdentifierNode, cmd parse.Node, args []parse.Node, final gjson.Result) gjson.Result {
	s.at(node)
	name := node.Ident

	// Handle built-in functions for gjson
	switch name {
	case "gjson":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}

		// Get the path argument
		pathArg := s.evalArg(dot, args[1])
		if pathArg.Type != gjson.String {
			s.errorf("gjson requires a string path argument")
		}

		// Use gjson's Get method with the path
		path := pathArg.String()
		result := dot.Get(path)

		// Check if the result exists
		if !result.Exists() && s.tmpl.option.missingKey == mapError {
			s.errorf("gjson path %q not found in data", path)
		}

		return result

	case "len":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}
		arg := s.evalArg(dot, args[1])
		if arg.IsArray() {
			return gjson.Parse(fmt.Sprintf("%d", len(arg.Array())))
		} else if arg.IsObject() {
			count := 0
			arg.ForEach(func(_, _ gjson.Result) bool {
				count++
				return true
			})
			return gjson.Parse(fmt.Sprintf("%d", count))
		} else if arg.Type == gjson.String {
			return gjson.Parse(fmt.Sprintf("%d", len(arg.String())))
		}
		return gjson.Parse("0")

	case "index":
		if len(args) < 3 {
			s.errorf("wrong number of args for %s: want at least 2 got %d", name, len(args)-1)
		}
		container := s.evalArg(dot, args[1])
		key := s.evalArg(dot, args[2])

		if container.IsArray() {
			// Check if key is a number
			if key.Type != gjson.Number {
				s.errorf("index of array must be integer")
			}
			idx := int(key.Int())
			arr := container.Array()
			if idx < 0 || idx >= len(arr) {
				s.errorf("array index out of range: %d", idx)
			}
			return arr[idx]
		} else if container.IsObject() {
			keyStr := key.String()
			return container.Get(keyStr)
		}
		s.errorf("can't index %s", container.Type)
		return gjson.Result{}

	case "print", "println":
		// These are handled by printValue, so we just evaluate and return the args
		var result strings.Builder
		for i := 1; i < len(args); i++ {
			arg := s.evalArg(dot, args[i])
			if i > 1 {
				result.WriteString(" ")
			}
			result.WriteString(arg.String())
		}
		if name == "println" {
			result.WriteString("\n")
		}
		return gjson.Parse(fmt.Sprintf("%q", result.String()))

	case "and", "or":
		// Short-circuit evaluation
		if len(args) < 2 {
			s.errorf("wrong number of args for %s: want at least 1 got 0", name)
		}

		var lastArg gjson.Result
		for i := 1; i < len(args); i++ {
			arg := s.evalArg(dot, args[i])
			lastArg = arg
			truth, ok := isGjsonTrue(arg)
			if !ok {
				s.errorf("%s can't use %v", name, arg.Raw)
			}

			if name == "and" {
				if !truth {
					return arg // Return the false value
				}
			} else { // or
				if truth {
					return arg // Return the true value
				}
			}
		}

		// Return the last argument
		return lastArg

	case "not":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}
		arg := s.evalArg(dot, args[1])
		truth, ok := isGjsonTrue(arg)
		if !ok {
			s.errorf("not can't use %v", arg.Raw)
		}
		return gjson.Parse(fmt.Sprintf("%t", !truth))

	case "eq", "ne", "lt", "le", "gt", "ge":
		if len(args) < 3 {
			s.errorf("wrong number of args for %s: want at least 2 got %d", name, len(args)-1)
		}

		arg1 := s.evalArg(dot, args[1])
		arg2 := s.evalArg(dot, args[2])

		// Compare based on the operation
		var result bool
		switch name {
		case "eq":
			// Special case for numbers
			if arg1.Type == gjson.Number && arg2.Type == gjson.Number {
				// Compare as numbers
				result = arg1.Num == arg2.Num
			} else if arg1.Type == gjson.Number && arg2.Type == gjson.String {
				// Try to convert string to number
				if num, err := strconv.ParseFloat(arg2.String(), 64); err == nil {
					result = arg1.Num == num
				} else {
					result = false
				}
			} else if arg1.Type == gjson.String && arg2.Type == gjson.Number {
				// Try to convert string to number
				if num, err := strconv.ParseFloat(arg1.String(), 64); err == nil {
					result = num == arg2.Num
				} else {
					result = false
				}
			} else {
				// Compare as strings or raw JSON
				result = arg1.Raw == arg2.Raw
			}
		case "ne":
			result = arg1.Raw != arg2.Raw
		case "lt":
			if arg1.Type == gjson.Number && arg2.Type == gjson.Number {
				result = arg1.Num < arg2.Num
			} else {
				result = arg1.String() < arg2.String()
			}
		case "le":
			if arg1.Type == gjson.Number && arg2.Type == gjson.Number {
				result = arg1.Num <= arg2.Num
			} else {
				result = arg1.String() <= arg2.String()
			}
		case "gt":
			if arg1.Type == gjson.Number && arg2.Type == gjson.Number {
				result = arg1.Num > arg2.Num
			} else {
				result = arg1.String() > arg2.String()
			}
		case "ge":
			if arg1.Type == gjson.Number && arg2.Type == gjson.Number {
				result = arg1.Num >= arg2.Num
			} else {
				result = arg1.String() >= arg2.String()
			}
		}

		return gjson.Parse(fmt.Sprintf("%t", result))

	case "html":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}
		arg := s.evalArg(dot, args[1])
		var b strings.Builder
		HTMLEscape(&b, []byte(arg.String()))
		return gjson.Parse(fmt.Sprintf("%q", b.String()))

	case "js":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}
		arg := s.evalArg(dot, args[1])
		var b strings.Builder
		JSEscape(&b, []byte(arg.String()))
		return gjson.Parse(fmt.Sprintf("%q", b.String()))

	case "urlquery":
		if len(args) != 2 {
			s.errorf("wrong number of args for %s: want 1 got %d", name, len(args)-1)
		}
		arg := s.evalArg(dot, args[1])
		return gjson.Parse(fmt.Sprintf("%q", url.QueryEscape(arg.String())))
	}

	// Special case for printf/sprintf
	if name == "printf" || name == "sprintf" {
		if len(args) < 2 {
			s.errorf("wrong number of args for %s: want at least 1 got %d", name, len(args)-1)
		}

		formatArg := s.evalArg(dot, args[1])
		if formatArg.Type != gjson.String {
			s.errorf("first argument to %s must be a format string", name)
		}

		format := formatArg.String()

		// Convert remaining arguments to Go values
		goArgs := make([]interface{}, 0, len(args)-2)
		for i := 2; i < len(args); i++ {
			arg := s.evalArg(dot, args[i])

			// Convert gjson.Result to appropriate Go value
			switch arg.Type {
			case gjson.Null:
				goArgs = append(goArgs, nil)
			case gjson.False, gjson.True:
				goArgs = append(goArgs, arg.Bool())
			case gjson.Number:
				// Check if it's an integer
				if arg.Num == float64(int64(arg.Num)) {
					goArgs = append(goArgs, int(arg.Int()))
				} else {
					goArgs = append(goArgs, arg.Float())
				}
			case gjson.String:
				goArgs = append(goArgs, arg.String())
			case gjson.JSON:
				if arg.IsArray() || arg.IsObject() {
					goArgs = append(goArgs, arg.Raw)
				} else {
					goArgs = append(goArgs, arg.Value())
				}
			}
		}

		// Format the string
		var result string
		if len(goArgs) > 0 {
			result = fmt.Sprintf(format, goArgs...)
		} else if final.Exists() {
			// Handle pipeline case where the final value is passed as an argument
			switch final.Type {
			case gjson.Null:
				result = fmt.Sprintf(format, nil)
			case gjson.False, gjson.True:
				result = fmt.Sprintf(format, final.Bool())
			case gjson.Number:
				// Check if it's an integer
				if final.Num == float64(int64(final.Num)) {
					result = fmt.Sprintf(format, int(final.Int()))
				} else {
					result = fmt.Sprintf(format, final.Float())
				}
			case gjson.String:
				result = fmt.Sprintf(format, final.String())
			case gjson.JSON:
				if final.IsArray() || final.IsObject() {
					result = fmt.Sprintf(format, final.Raw)
				} else {
					result = fmt.Sprintf(format, final.Value())
				}
			}
		} else {
			// No arguments provided
			result = format
		}

		return gjson.Parse(fmt.Sprintf("%q", result))
	}

	// Try to find the function in the template's function map or builtins
	fn, _, found := findFunction(name, s.tmpl)
	if found && name != "printf" && name != "sprintf" {
		// Convert gjson.Result arguments to reflect.Value
		reflectArgs := make([]reflect.Value, 0, len(args)-1)
		for i := 1; i < len(args); i++ {
			arg := s.evalArg(dot, args[i])
			var reflectArg reflect.Value

			// Convert gjson.Result to appropriate reflect.Value based on type
			switch arg.Type {
			case gjson.Null:
				reflectArg = reflect.Zero(reflect.TypeOf((*any)(nil)).Elem())
			case gjson.False, gjson.True:
				reflectArg = reflect.ValueOf(arg.Bool())
			case gjson.Number:
				// Check if it's an integer
				if arg.Num == float64(int64(arg.Num)) {
					reflectArg = reflect.ValueOf(int(arg.Int()))
				} else {
					reflectArg = reflect.ValueOf(arg.Float())
				}
			case gjson.String:
				reflectArg = reflect.ValueOf(arg.String())
			case gjson.JSON:
				// For JSON objects/arrays, we'll pass the raw JSON string
				reflectArg = reflect.ValueOf(arg.Raw)
			}

			reflectArgs = append(reflectArgs, reflectArg)
		}

		// Call the function
		result, err := safeCall(fn, reflectArgs)
		if err != nil {
			s.errorf("%s: %s", name, err)
		}

		// Convert the result back to gjson.Result
		switch result.Kind() {
		case reflect.Bool:
			return gjson.Parse(fmt.Sprintf("%t", result.Bool()))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return gjson.Parse(fmt.Sprintf("%d", result.Int()))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return gjson.Parse(fmt.Sprintf("%d", result.Uint()))
		case reflect.Float32, reflect.Float64:
			return gjson.Parse(fmt.Sprintf("%f", result.Float()))
		case reflect.String:
			return gjson.Parse(fmt.Sprintf("%q", result.String()))
		case reflect.Slice, reflect.Array:
			if result.Type().Elem().Kind() == reflect.Uint8 {
				// []byte
				return gjson.Parse(fmt.Sprintf("%q", string(result.Bytes())))
			}
			// Fall through to default
		}

		// For other types, convert to string
		return gjson.Parse(fmt.Sprintf("%q", fmt.Sprint(result.Interface())))
	}

	// If we get here, the function was not found
	s.errorf("function %q not implemented for gjson", name)
	return gjson.Result{}
}

// evalField evaluates an expression like (.Field) or (.Field arg1 arg2).
// The 'final' argument represents the return value from the preceding
// value of the pipeline, if any.
func (s *state) evalField(dot gjson.Result, fieldName string, node parse.Node, args []parse.Node, final, receiver gjson.Result) gjson.Result {
	if !receiver.Exists() {
		if s.tmpl.option.missingKey == mapError { // Treat invalid value as missing map key.
			s.errorf("nil data; no entry for key %q", fieldName)
		}
		return gjson.Result{}
	}

	// Check if it's a method call with arguments
	hasArgs := len(args) > 1 || final.Exists()
	if hasArgs {
		s.errorf("%s is not a method but has arguments", fieldName)
	}

	// Use gjson's native Get method to retrieve the value
	result := receiver.Get(fieldName)

	// Check if the result exists
	if !result.Exists() {
		switch s.tmpl.option.missingKey {
		case mapInvalid:
			// Just use the invalid value.
		case mapZeroValue:
			// Return empty result
			return gjson.Result{}
		case mapError:
			s.errorf("path %q not found in data", fieldName)
		}
	}

	return result
}

// isNumeric checks if a string is a valid number
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

// evalArg evaluates a single argument for a function
func (s *state) evalArg(dot gjson.Result, n parse.Node) gjson.Result {
	s.at(n)
	switch arg := n.(type) {
	case *parse.DotNode:
		return dot
	case *parse.NilNode:
		return gjson.Parse("null")
	case *parse.FieldNode:
		return s.evalFieldNode(dot, arg, []parse.Node{n}, gjson.Result{})
	case *parse.VariableNode:
		return s.evalVariableNode(dot, arg, nil, gjson.Result{})
	case *parse.PipeNode:
		return s.evalPipeline(dot, arg)
	case *parse.IdentifierNode:
		return s.evalFunction(dot, arg, arg, nil, gjson.Result{})
	case *parse.ChainNode:
		return s.evalChainNode(dot, arg, nil, gjson.Result{})
	case *parse.BoolNode:
		return gjson.Parse(fmt.Sprintf("%t", arg.True))
	case *parse.NumberNode:
		return s.idealConstantGjson(arg)
	case *parse.StringNode:
		return gjson.Parse(fmt.Sprintf("%q", arg.Text))
	}
	s.errorf("can't handle %s for arg", n)
	return gjson.Result{}
}

var (
	errorType        = reflect.TypeFor[error]()
	fmtStringerType  = reflect.TypeFor[fmt.Stringer]()
	reflectValueType = reflect.TypeFor[reflect.Value]()
)

// 删除旧的反射相关方法，因为我们已经使用gjson替代了它们

// printValue writes the textual representation of the value to the output of
// the template.
func (s *state) printValue(n parse.Node, v gjson.Result) {
	s.at(n)
	var output string

	// Special case for missing values
	if !v.Exists() {
		// For compatibility with the original template package, return empty string
		output = ""
	} else {
		switch v.Type {
		case gjson.String:
			// For strings, we want to print without the quotes
			output = v.String()
		case gjson.JSON:
			if v.IsObject() || v.IsArray() {
				// For objects and arrays, we want to print the JSON
				output = v.Raw
			} else {
				output = v.String()
			}
		default:
			// For other types, just use the raw value
			output = v.Raw
		}
	}

	_, err := fmt.Fprint(s.wr, output)
	if err != nil {
		s.writeError(err)
	}
}

// indirect returns the item at the end of indirection, and a bool to indicate
// if it's nil. If the returned bool is true, the returned value's kind will be
// either a pointer or interface.
func indirect(v reflect.Value) (rv reflect.Value, isNil bool) {
	for ; v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface; v = v.Elem() {
		if v.IsNil() {
			return v, true
		}
	}
	return v, false
}

// indirectInterface returns the concrete value in an interface value,
// or else the zero reflect.Value.
// That is, if v represents the interface value x, the result is the same as reflect.ValueOf(x):
// the fact that x was an interface value is forgotten.
func indirectInterface(v reflect.Value) reflect.Value {
	if v.Kind() != reflect.Interface {
		return v
	}
	if v.IsNil() {
		return reflect.Value{}
	}
	return v.Elem()
}

// canBeNil reports whether an untyped nil can be assigned to the type. See reflect.Zero.
func canBeNil(typ reflect.Type) bool {
	switch typ.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	case reflect.Struct:
		return typ == reflectValueType
	}
	return false
}
