# Understanding JSON and Templates

*Published on 2025-03-30 by Jane Smith*


> **Popular Article**: This post has been viewed over 1250 times!


## About the Author

**Jane Smith** - Software engineer with 10+ years of experience

Contact: [jane@example.com](mailto:jane@example.com)

Follow on: [Twitter](https://twitter.com/@janesmith) and [GitHub](https://github.com/janesmith)


## Introduction

This article explores the power of combining JSON with templates.

## Table of Contents


1. [JSON Basics](#json-basics)

2. [Templates in Go](#templates-in-go)

3. [Advanced GJSON Features](#advanced-gjson-features)



## JSON Basics

JSON (JavaScript Object Notation) is a lightweight data-interchange format...


### Code Examples


```json
{ "name": "John", "age": 30 }
```

```go
data := map[string]interface{}{"name": "John", "age": 30}
```




> **Note**: This section is particularly important!



## Templates in Go

Go's template package provides powerful text processing capabilities...


### Code Examples


```go
tmpl, _ := template.New("test").Parse("Hello {{.Name}}!")
```






## Advanced GJSON Features

GJSON provides advanced JSON parsing capabilities with a simple syntax...


### Code Examples


```go
result := gjson.Get(json, "users.#(name=John).age")
```




> **Note**: This section is particularly important!




## Comments (3)


### Bob Johnson - 2025-03-31 (5/5 stars)

Great article! I learned a lot about templates.


**Replies:**


- **Jane Smith** (2025-03-31): Thanks Bob! Glad you enjoyed it.




### Alice Williams - 2025-04-01 (4/5 stars)

I'd love to see more examples of advanced template usage.




### Charlie Brown - 2025-04-02 (5/5 stars)

This helped me solve a problem I was having with JSON parsing.





## Statistics

- Views: 1250
- Likes: 42
- Shares: 17


This article is highly rated with 42 likes!



This article has 3 comments!


## GJSON Advanced Features

### Array and Object Modifiers

**Keys of author's social profiles**: ["twitter","github"]

**Values of author's social profiles**: ["@janesmith","janesmith"]

**Reversed tags**: ["markdown","golang","templates","json"]

**First two tags**: 

### Array Queries

**5-star comments**: ["Bob Johnson","Charlie Brown"]

**Comments with replies**: ["Bob Johnson"]

**Sections with high importance**: ["JSON Basics","Advanced GJSON Features"]

### Multipaths

**Author info**: {"name":"Jane Smith","email":"jane@example.com"}

**Blog stats summary**: {"views":1250,"likes":42}

### Literals and Wildcards

**Custom JSON**: {"author":"Jane Smith","rating":"Excellent","date":1648684800000}

**Wildcard section search**: JSON (JavaScript Object Notation) is a lightweight data-interchange format...
