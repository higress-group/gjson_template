package main

import (
	"bytes"
	"fmt"
	"log"
	"os"

	template "github.com/higress-group/gjson_template"
)

const jsonData = `{
	"blog": {
		"title": "Understanding JSON and Templates",
		"author": {
			"name": "Jane Smith",
			"email": "jane@example.com",
			"bio": "Software engineer with 10+ years of experience",
			"social": {
				"twitter": "@janesmith",
				"github": "janesmith"
			}
		},
		"published": "2025-03-30",
		"tags": ["json", "templates", "golang", "markdown"],
		"content": {
			"intro": "This article explores the power of combining JSON with templates.",
			"sections": [
				{
					"title": "JSON Basics",
					"content": "JSON (JavaScript Object Notation) is a lightweight data-interchange format...",
					"code_examples": [
						{"language": "json", "code": "{ \"name\": \"John\", \"age\": 30 }"},
						{"language": "go", "code": "data := map[string]interface{}{\"name\": \"John\", \"age\": 30}"}
					],
					"importance": 5
				},
				{
					"title": "Templates in Go",
					"content": "Go's template package provides powerful text processing capabilities...",
					"code_examples": [
						{"language": "go", "code": "tmpl, _ := template.New(\"test\").Parse(\"Hello {{.Name}}!\")"}
					],
					"importance": 4
				},
				{
					"title": "Advanced GJSON Features",
					"content": "GJSON provides advanced JSON parsing capabilities with a simple syntax...",
					"code_examples": [
						{"language": "go", "code": "result := gjson.Get(json, \"users.#(name=John).age\")"}
					],
					"importance": 5
				}
			]
		},
		"comments": [
			{
				"user": "Bob Johnson",
				"date": "2025-03-31",
				"text": "Great article! I learned a lot about templates.",
				"rating": 5,
				"replies": [
					{
						"user": "Jane Smith",
						"date": "2025-03-31",
						"text": "Thanks Bob! Glad you enjoyed it."
					}
				]
			},
			{
				"user": "Alice Williams",
				"date": "2025-04-01",
				"text": "I'd love to see more examples of advanced template usage.",
				"rating": 4,
				"replies": []
			},
			{
				"user": "Charlie Brown",
				"date": "2025-04-02",
				"text": "This helped me solve a problem I was having with JSON parsing.",
				"rating": 5,
				"replies": []
			}
		],
		"stats": {
			"views": 1250,
			"likes": 42,
			"shares": 17
		}
	}
}`

const markdownTemplate = `# {{.blog.title}}

*Published on {{.blog.published}} by {{.blog.author.name}}*

{{if gt (gjson "blog.stats.views") 1000}}
> **Popular Article**: This post has been viewed over {{.blog.stats.views}} times!
{{end}}

## About the Author

**{{.blog.author.name}}** - {{.blog.author.bio}}

Contact: [{{.blog.author.email}}](mailto:{{.blog.author.email}})
{{if .blog.author.social}}
Follow on: {{if .blog.author.social.twitter}}[Twitter](https://twitter.com/{{.blog.author.social.twitter}}){{end}}{{if and .blog.author.social.twitter .blog.author.social.github}} and {{end}}{{if .blog.author.social.github}}[GitHub](https://github.com/{{.blog.author.social.github}}){{end}}
{{end}}

## Introduction

{{.blog.content.intro}}

## Table of Contents

{{range $index, $section := .blog.content.sections}}
{{$num := add $index 1}}. [{{$section.title}}](#{{$section.title}})
{{end}}

{{range $index, $section := .blog.content.sections}}
## {{$section.title}}

{{$section.content}}

{{if $section.code_examples}}
### Code Examples

{{range $example := $section.code_examples}}
` + "```{{$example.language}}\n{{$example.code}}\n```" +
	`
{{end}}
{{end}}

{{if gt $section.importance 4}}
> **Note**: This section is particularly important!
{{end}}

{{end}}

## Tags

{{range $index, $tag := .blog.tags}}
{{if $index}}, {{end}}#{{$tag}}
{{end}}

## Comments ({{len .blog.comments}})

{{range $comment := .blog.comments}}
### {{$comment.user}} - {{$comment.date}} {{if $comment.rating}}({{$comment.rating}}/5 stars){{end}}

{{$comment.text}}

{{if $comment.replies}}
**Replies:**

{{range $reply := $comment.replies}}
- **{{$reply.user}}** ({{$reply.date}}): {{$reply.text}}
{{end}}
{{end}}

{{end}}

## Statistics

- Views: {{.blog.stats.views}}
- Likes: {{.blog.stats.likes}}
- Shares: {{.blog.stats.shares}}

{{if gt .blog.stats.likes 40}}
This article is highly rated with {{.blog.stats.likes}} likes!
{{end}}

{{if gt (len .blog.comments) 0}}
This article has {{len .blog.comments}} comments!
{{end}}

## GJSON Advanced Features

### Array and Object Modifiers

**Keys of author's social profiles**: {{gjson "blog.author.social.@keys"}}

**Values of author's social profiles**: {{gjson "blog.author.social.@values"}}

**Reversed tags**: {{gjson "blog.tags.@reverse"}}

**First two tags**: {{gjson "blog.tags.@slice:0:2"}}

### Array Queries

**5-star comments**: {{gjson "blog.comments.#(rating==5)#.user"}}

**Comments with replies**: {{gjson "blog.comments.#(replies.#>0)#.user"}}

**Sections with high importance**: {{gjson "blog.content.sections.#(importance>4)#.title"}}

### Multipaths

**Author info**: {{gjson "{\"name\":blog.author.name,\"email\":blog.author.email}"}}

**Blog stats summary**: {{gjson "{\"views\":blog.stats.views,\"likes\":blog.stats.likes}"}}

### Literals and Wildcards

**Custom JSON**: {{gjson "{\"author\":blog.author.name,\"rating\":!\"Excellent\",\"date\":!1648684800000}"}}

**Wildcard section search**: {{gjson "blog.content.sections.#(title%\"*Basics\").content"}}
`

var customFuncs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
	"lower": func(s string) string {
		return string(bytes.ToLower([]byte(s)))
	},
	"replace": func(s, old, new string) string {
		return string(bytes.ReplaceAll([]byte(s), []byte(old), []byte(new)))
	},
	"trimPrefix": func(s, prefix string) string {
		return string(bytes.TrimPrefix([]byte(s), []byte(prefix)))
	},
}

func main() {
	tmpl, err := template.New("markdown").Funcs(customFuncs).Parse(markdownTemplate)
	if err != nil {
		log.Fatalf("Error parsing template: %v", err)
	}

	data := []byte(jsonData)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		log.Fatalf("Error executing template: %v", err)
	}

	fmt.Println("Markdown generated successfully!")

	if err := os.WriteFile("output.md", buf.Bytes(), 0644); err != nil {
		log.Fatalf("Error writing to file: %v", err)
	}

	fmt.Println("Markdown saved to output.md")
}
