# GJSON Template

A powerful template engine based on Go's standard template library, enhanced with GJSON path syntax support for more flexible and efficient JSON processing without reflection.

## Overview

GJSON Template is a fork of Go's standard template package that replaces the reflection-based value lookup mechanism with [GJSON](https://github.com/tidwall/gjson) path syntax. This provides several advantages:

- **Performance**: Eliminates the overhead of reflection when working with JSON data by avoiding unmarshal operations, reducing small object allocations and GC pressure
- **Flexibility**: Leverages GJSON's powerful path syntax for complex JSON queries
- **Simplicity**: Maintains the familiar Go template syntax while adding powerful JSON capabilities

The library accepts JSON data as `[]byte` instead of `interface{}`, allowing for direct parsing with GJSON while preserving all the template features like conditionals, loops, and variable assignments.

## Installation

```bash
go get github.com/higress-group/gjson_template
```

## Basic Usage

Here's a simple example demonstrating how to use GJSON Template:

```go
package main

import (
    "fmt"
    "os"
    
    template "github.com/higress-group/gjson_template"
)

func main() {
    // JSON data as a string
    jsonData := []byte(`{
        "name": "John Doe",
        "age": 30,
        "address": {
            "city": "New York",
            "country": "USA"
        },
        "skills": ["Go", "JavaScript", "Python"]
    }`)
    
    // Create a template with GJSON path syntax
    tmpl, err := template.New("example").Parse(`
        Name: {{.name}}
        Age: {{.age}}
        City: {{.address.city}}
        First Skill: {{.skills.0}}
        All Skills: {{range .skills}}{{.}}, {{end}}
    `)
    
    if err != nil {
        fmt.Printf("Error parsing template: %v\n", err)
        return
    }
    
    // Execute the template with JSON data
    err = tmpl.Execute(os.Stdout, jsonData)
    if err != nil {
        fmt.Printf("Error executing template: %v\n", err)
    }
}
```

## Advanced GJSON Path Features

GJSON Template supports all of GJSON's powerful path syntax. Here's an example showcasing some advanced features:

```go
package main

import (
    "fmt"
    "os"
    
    template "github.com/higress-group/gjson_template"
)

func main() {
    // JSON data with more complex structure
    jsonData := []byte(`{
        "users": [
            {"name": "Alice", "age": 28, "active": true, "roles": ["admin", "developer"]},
            {"name": "Bob", "age": 35, "active": false, "roles": ["developer"]},
            {"name": "Charlie", "age": 42, "active": true, "roles": ["manager", "developer"]}
        ],
        "settings": {
            "theme": "dark",
            "notifications": {
                "email": true,
                "sms": false
            }
        }
    }`)
    
    // Template with advanced GJSON path syntax
    tmpl, err := template.New("advanced").Parse(`
        <!-- Using array indexing -->
        First user: {{.users.0.name}}
        
        <!-- Using the gjson function for more complex queries -->
        Active users: {{gjson "users.#(active==true)#.name"}}
        
        <!-- Array filtering with multiple conditions -->
        Active developers over 30: {{gjson "users.#(active==true && age>30)#.name"}}
        
        <!-- Using modifiers -->
        User names (reversed): {{gjson "users.@reverse.#.name"}}
        
        <!-- Working with nested properties -->
        Email notifications: {{if .settings.notifications.email}}Enabled{{else}}Disabled{{end}}
        
        <!-- Iterating over filtered results -->
        Admins:
        {{range $user := gjson "users.#(roles.#(==admin)>0)#"}}
          - {{$user.name}} ({{$user.age}})
        {{end}}
    `)
    
    if err != nil {
        fmt.Printf("Error parsing template: %v\n", err)
        return
    }
    
    err = tmpl.Execute(os.Stdout, jsonData)
    if err != nil {
        fmt.Printf("Error executing template: %v\n", err)
    }
}
```

For even more complex examples, check out the [markdown.go](example/markdown.go) file in the example directory, which demonstrates how to convert a JSON blog post into a Markdown document using advanced GJSON path features.

## GJSON Path Syntax

GJSON Template supports the full GJSON path syntax. Here are some key features:

- **Dot notation**: `address.city`
- **Array indexing**: `users.0.name`
- **Array iteration**: `users.#.name`
- **Wildcards**: `users.*.name`
- **Array filtering**: `users.#(age>=30)#.name`
- **Modifiers**: `users.@reverse.#.name`
- **Multipath**: `{name:users.0.name,count:users.#}`
- **Escape characters**: `path.with\.dot`

For a complete reference of GJSON path syntax, see the [GJSON documentation](https://github.com/tidwall/gjson#path-syntax).

## Built-in Functions with Sprig

GJSON Template comes with all of [Sprig](https://github.com/Masterminds/sprig)'s functions built-in, providing a rich set of over 70 template functions for string manipulation, math operations, date formatting, list processing, and more. This makes GJSON Template functionally equivalent to Helm's template capabilities.

Some commonly used Sprig functions include:

- **String manipulation**: `trim`, `upper`, `lower`, `replace`, `plural`, `nospace`
- **Math operations**: `add`, `sub`, `mul`, `div`, `max`, `min`
- **Date formatting**: `now`, `date`, `dateInZone`, `dateModify`
- **List operations**: `list`, `first`, `last`, `uniq`, `sortAlpha`
- **Dictionary operations**: `dict`, `get`, `set`, `hasKey`, `pluck`
- **Flow control**: `ternary`, `default`, `empty`, `coalesce`
- **Type conversion**: `toString`, `toJson`, `toPrettyJson`, `toRawJson`
- **Encoding/decoding**: `b64enc`, `b64dec`, `urlquery`, `urlqueryescape`
- **UUID generation**: `uuidv4`

Example usage:

```go
// String manipulation
{{lower .title | replace " " "-"}}  // Convert to lowercase and replace spaces with hyphens

// Math operations
{{add 5 .count}}  // Add 5 to the count value

// Date formatting
{{now | date "2006-01-02"}}  // Format current date as YYYY-MM-DD

// List operations
{{list 1 2 3 | join ","}}  // Create a list and join with commas
```

For a complete reference of all available functions, see the [Helm documentation on functions](https://helm.sh/docs/chart_template_guide/function_list/), as GJSON Template includes the same function set.

## AI Prompt for Template Generation

When working with AI assistants to generate templates using GJSON Template, you can use the following prompt to help the AI understand the syntax:

```
When generating Go templates for the GJSON Template library, please follow these guidelines:

1. The library is based on Go's standard template package but uses GJSON for JSON parsing.

2. JSON data is passed as []byte instead of interface{}, and values are accessed using GJSON path syntax.

3. Basic dot notation works as in standard templates: {{.user.name}}

4. For complex queries, use the gjson function:
   - Simple path: {{gjson "users.0.name"}}
   - Array filtering: {{gjson "users.#(active==true)#.name"}}
   - Using modifiers: {{gjson "users.@reverse.#.name"}}
   - Multipath: {{gjson "{name:users.0.name,count:users.#}"}}

5. All standard template features work normally:
   - Conditionals: {{if .condition}}...{{else}}...{{end}}
   - Loops: {{range .items}}...{{end}}
   - Variables: {{$var := .value}}
   - Functions: {{len .array}}

6. When using backticks for GJSON paths, escape any internal quotes:
   {{gjson `users.#(name=="John").age`}}

7. For complex paths with special characters, use double quotes with escaping:
   {{gjson "path.with\\.dot"}}

Please generate a template that processes JSON data according to these guidelines.
```
