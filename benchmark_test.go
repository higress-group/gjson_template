package gjson_template_test

import (
	"bytes"
	"encoding/json"
	"testing"
	stdtemplate "text/template"

	gjsontemplate "github.com/higress-group/gjson_template"
)

// Simple JSON data for testing
var simpleJSON = []byte(`{
	"name": "John Doe",
	"age": 30,
	"email": "john@example.com",
	"address": {
		"city": "New York",
		"country": "USA"
	},
	"tags": ["developer", "golang", "json"]
}`)

// Complex JSON data for testing
var complexJSON = []byte(`{
	"users": [
		{
			"id": 1,
			"name": "Alice",
			"email": "alice@example.com",
			"active": true,
			"roles": ["admin", "developer"],
			"metadata": {
				"lastLogin": "2025-03-30T10:00:00Z",
				"preferences": {
					"theme": "dark",
					"notifications": true
				}
			},
			"posts": [
				{
					"id": 101,
					"title": "Introduction to GJSON",
					"tags": ["json", "golang", "tutorial"],
					"comments": [
						{
							"user": "Bob",
							"text": "Great article!",
							"likes": 5
						},
						{
							"user": "Charlie",
							"text": "Very helpful, thanks!",
							"likes": 3
						}
					]
				},
				{
					"id": 102,
					"title": "Advanced Template Techniques",
					"tags": ["templates", "golang", "advanced"],
					"comments": [
						{
							"user": "Dave",
							"text": "This saved me hours of work",
							"likes": 10
						}
					]
				}
			]
		},
		{
			"id": 2,
			"name": "Bob",
			"email": "bob@example.com",
			"active": false,
			"roles": ["developer"],
			"metadata": {
				"lastLogin": "2025-03-29T15:30:00Z",
				"preferences": {
					"theme": "light",
					"notifications": false
				}
			},
			"posts": []
		}
	],
	"stats": {
		"totalUsers": 2,
		"activeUsers": 1,
		"totalPosts": 2,
		"totalComments": 3,
		"averageLikes": 6
	}
}`)

// Simple template - same for both libraries
const simpleTemplate = `
Name: {{.name}}
Age: {{.age}}
Email: {{.email}}
City: {{.address.city}}
Country: {{.address.country}}
Tags: {{range $index, $tag := .tags}}{{if $index}}, {{end}}{{$tag}}{{end}}
`

// Complex template - same for both libraries
const complexTemplate = `
{{range $user := .users}}
# User Profile: {{$user.name}}

Email: {{$user.email}}
Status: {{if $user.active}}Active{{else}}Inactive{{end}}
Roles: {{range $index, $role := $user.roles}}{{if $index}}, {{end}}{{$role}}{{end}}
Last Login: {{$user.metadata.lastLogin}}
Theme: {{$user.metadata.preferences.theme}}

## Posts
{{range $post := $user.posts}}
### {{$post.title}}

Tags: {{range $index, $tag := $post.tags}}{{if $index}}, {{end}}{{$tag}}{{end}}

Comments:
{{range $comment := $post.comments}}
- {{$comment.user}}: {{$comment.text}} ({{$comment.likes}} likes)
{{end}}
{{else}}
No posts yet.
{{end}}
{{end}}

# Statistics
Total Users: {{.stats.totalUsers}}
Active Users: {{.stats.activeUsers}}
Total Posts: {{.stats.totalPosts}}
Total Comments: {{.stats.totalComments}}
Average Likes: {{.stats.averageLikes}}
`

// Pre-parse templates to avoid including parsing time in execution benchmarks
var simpleGJSONTmpl *gjsontemplate.Template
var complexGJSONTmpl *gjsontemplate.Template
var simpleStdTmpl *stdtemplate.Template
var complexStdTmpl *stdtemplate.Template

func init() {
	var err error

	// Parse templates once to avoid including parsing time in execution benchmarks
	simpleGJSONTmpl, err = gjsontemplate.New("simple").Parse(simpleTemplate)
	if err != nil {
		panic(err)
	}

	complexGJSONTmpl, err = gjsontemplate.New("complex").Parse(complexTemplate)
	if err != nil {
		panic(err)
	}

	simpleStdTmpl, err = stdtemplate.New("simple").Parse(simpleTemplate)
	if err != nil {
		panic(err)
	}

	complexStdTmpl, err = stdtemplate.New("complex").Parse(complexTemplate)
	if err != nil {
		panic(err)
	}
}

// Benchmark: Simple template with GJSON Template (including JSON parsing)
func BenchmarkSimpleGJSONTemplate(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		err := simpleGJSONTmpl.Execute(&buf, simpleJSON)
		if err != nil {
			b.Fatalf("Template execution failed: %v", err)
		}
	}
}

// Benchmark: Simple template with standard template library (including JSON parsing)
func BenchmarkSimpleStdTemplate(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Parse JSON for each iteration to include parsing time
		var data map[string]interface{}
		err := json.Unmarshal(simpleJSON, &data)
		if err != nil {
			b.Fatalf("JSON parsing failed: %v", err)
		}

		buf.Reset()
		err = simpleStdTmpl.Execute(&buf, data)
		if err != nil {
			b.Fatalf("Template execution failed: %v", err)
		}
	}
}

// Benchmark: Complex template with GJSON Template (including JSON parsing)
func BenchmarkComplexGJSONTemplate(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		buf.Reset()
		err := complexGJSONTmpl.Execute(&buf, complexJSON)
		if err != nil {
			b.Fatalf("Template execution failed: %v", err)
		}
	}
}

// Benchmark: Complex template with standard template library (including JSON parsing)
func BenchmarkComplexStdTemplate(b *testing.B) {
	var buf bytes.Buffer
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Parse JSON for each iteration to include parsing time
		var data map[string]interface{}
		err := json.Unmarshal(complexJSON, &data)
		if err != nil {
			b.Fatalf("JSON parsing failed: %v", err)
		}

		buf.Reset()
		err = complexStdTmpl.Execute(&buf, data)
		if err != nil {
			b.Fatalf("Template execution failed: %v", err)
		}
	}
}

// Benchmark: Template parsing - GJSON Template
func BenchmarkParseGJSONTemplate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := gjsontemplate.New("complex").Parse(complexTemplate)
		if err != nil {
			b.Fatalf("Failed to parse template: %v", err)
		}
	}
}

// Benchmark: Template parsing - standard template library
func BenchmarkParseStdTemplate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := stdtemplate.New("complex").Parse(complexTemplate)
		if err != nil {
			b.Fatalf("Failed to parse template: %v", err)
		}
	}
}
