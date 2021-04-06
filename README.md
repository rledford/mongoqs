# mongoqs

`mongoqs` is a URL query string processor that converts query strings to MongoDB queries.

## Table of Contents

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [Query Fields](#query-fields)
  - [Reserved Fields](#reserved-fields)
- [URL Query Syntax](#url-query-syntax)
  - [Operators](#operators)
  - [Query String Examples](#Query-string-examples)
    - [Equal](#equal)
    - [Range](#range)
    - [In](#in)
    - [Not In](#not-in)
    - [All](#all)
    - [Nested](#nested)
- [Query Result](#query-result)
  - [Filter](#filter)
  - [Projection](#projection)
  - [Sort](#sorts)
  - [Limit](#limit)
  - [Skip](#skip)
- [Examples](#examples)
- [Shortcomings](#shortcomings)
- [Backlog](#backlog)

## Features

- Simple query string syntax
- Easy configuration
  - Query field configuration methods are chainable
- Supports common MongoDB operators
- Ensures only the configured fields appear in resulting MongoDB filter
- Supports projections
- Supports multiple operators within a single field in a query string
- Allows one or more aliases for each field
- Allows default functions that are used to set filters for fields that are missing or invalid
- Supports safe regex searches
  - Search fields that start with a sequence
  - Search fields that end with a sequence
  - Search fields that contain a sequence
  - Uses Go's regex.QuoteMeta method to escape regex operators that appear in the query string to prevent potentially unsafe regular expressions from being executed

## Install

To install _mongoqs_, first make sure Go **version 1.16+** is installed and your Go workspace is set.

1. Add _mongoqs_ to your Go project dependencies

```bash
$ go get -u github.com/rledford/mongoqs
```

2. Import _mongoqs_ into your code

```go
import "github.com/rledford/mongoqs"
```

## Usage

```go
import (
  "fmt" // for usage exmaple only
  "net/url" // for usage example only

  "github.com/rledford/mongoqs"
)
// create and configure query fields
myStringField := mongoqs.NewQField("myString", mongoqs.QString)
myIntField := mongoqs.NewQField("myInt", mongoqs.QInt)
myIntField.IsSortable() // allow this field to be used in sorts
myIntField.IsProjectable() // // allow this field to be used in projections
myFloatField := mongoqs.NewQField("myFloat", mongoqs.QFloat)
myFloatField.IsSortable().IsProjectable() // same as calls on myIntField but chained
myBoolField := mongoqs.NewQField("myBool", mongoqs.QBool)
myDateTimeField := mongoqs.NewQField("myDateTime", mongoqs.QDateTime)
myObjectIDField := mongoqs.NewQField("myObjectID", mongoqs.QObjectID)
myObjectIDField.UseAlias("_id", "id") // will use _id and id to refer to myObjectID
// create a new query processor
qproc := mongoqs.NewQProcessor(myStringField, myIntField, myFloatField, myBoolField, myDateTimeField, myObjectIDField)

// we'll use the net/url package's Values to construct query to process, but it would be more common to use on from an http request
qs := url.Values{}
qs.Add("unknown", "nin:1,2,3,4") // a QField was not created for 'unknown' so it will be ignored
qs.Add("myString", "in:a,b,c,d")
qs.Add("myInt", "gt:1,lt:10")
qs.Add("myFloat", "1.0") // 'eq:' operator is assumed
qs.Add("myBool", "false") // 'eq:' operator is assumed
qs.Add("myDateTimeField", "gte:2021-01-01T15:00:00Z")
qs.Add("id", "6050e7f529a90b22dc47f19e") // using an alias of myObjectID ('eq:' operator is assumed)
qs.Add("srt", "-myInt,+myString") // sort by myInt in descending order (myStringField.IsSortable() was not called so +myString is ignored)
qs.Add("prj", "-myFloat") // exclude myFloat field from query results
qs.Add("lmt", "10") // limit to 10 results
qs.Add("skp", "100") // skip the first 100 results

result, err := qproc(qs)
if err == nil {
  fmt.Println(result.String())
}
```

### Output

```bash
--- Filter ---
map[myBool:map[$eq:false] myFloat:map[$eq:1] myInt:map[$gt:1 $lt:10] myObjectID:map[$eq:ObjectID("6050e7f529a90b22dc47f19e")] myString:map[$in:[a b c d]]]
--------------
--- Projection ---
map[myFloat:0]
------------------
--- Sort ---
map[myInt:-1]
-------------
--- Paging ---
Limit:  10
Skip:   100
--------------
```

## Query Fields

Query fields (QField) are used to build query processors (QProcessor). It is recommended to use the _NewQueryField_ method when creating a new QField.

| Property    | Type            | Description                                                                                                                                      |
| ----------- | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ |
| Name        | string          | The name of the field. This should match the target field in the database schema.                                                                |
| Type        | QType           | The type used when parsing query strings for this field                                                                                          |
| Default     | \*func() bson.M | An optional pointer to a default function that will be used to set the filter for this field if the field is missing or if the value is invalid. |
| Aliases     | []string        | A slice of strings that can be used as aliases for the field's name.                                                                             |
| Projectable | Bool            | Whether the field is allowed in projections or not.                                                                                              |
| Sortable    | Bool            | Whether the field is allowed to be used to sort or not.                                                                                          |

### Reserved Fields

## Syntax

### Operators

### Query String Examples

## Query Result

### Filter

### Projection

### Sort

### Limit

### Skip

## Examples

## Backlog

- Nested wild card fields
  - Field names will be able to be defined as `field.*` or `field.*.nested` (not `field.*.*` though). This will allow querying nested document fields that may be dynamically set.
