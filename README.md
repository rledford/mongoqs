# MongoQS

MongoQS is a URL query string processor that converts query strings to MongoDB query filters and options.

## Table of Contents

- [Features](#features)
- [Install](#install)
- [Usage](#usage)
- [QField](#qfield)
  - [Reserved Keys](#reserved-keys)
  - [Comparison Operators](#comparison-operators)
  - [Sort Operators](#sort-operators)
  - [Projection Operators](#projection-operators)
  - [Methods](#qfield-methods)
  - [About Meta Fields](#about-meta-fields)
- [Query Strings](#query-strings)
  - [Syntax](#syntax)
  - [Equal To](#equal-to)
  - [Not Equal To](#not-equal-to)
  - [Greater Than, Less Than](#greater-than-less-than)
  - [Greater Than Equal To, Less Than Equal To](#greater-than-equal-to-less-than-equal-to)
  - [In, Not In, All](#in-not-in-all)
  - [Like, Starts Like, Ends Like](#like-starts-like-ends-like)
  - [Mixed](#mixed)
- [QResult](#qresult)
- [Backlog](#backlog)

## Features

- Simple query string syntax
- Easy configuration
  - Query field configuration methods are chainable
- Supports common MongoDB operators
- Ensures only the configured fields appear in resulting MongoDB filter
- Supports projections
- Supports multiple operators on a single field in a query string
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
$ go get -u github.com/rledford/mongoqs@latest
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

  mqs "github.com/rledford/mongoqs"
)
// create and configure query fields
myStringField := NewQField("myString") // QType String is default so no call to ParseAsString is necessary
myStringFieldWithDefault := NewQField("myStringWithDefault")
myStringFieldWithDefault.UseDefault(func() string { return "slike:Something useful" }) // provide function that returns a string that uses MongoQS syntax
myIntField := NewQField("myInt")
myIntField.ParseAsInt() // parse query string values integers
myIntField.Sortable() // allow this field to be used in sorts
myIntField.Projectable() // allow this field to be used in projections
myFloatField := NewQField("myFloat")
myFloatField.ParseAsFloat().Sortable().Projectable() // chained
myBoolField := NewQField("myBool")
myBoolField.ParseAsBool()
myDateTimeField := NewQField("myDateTime")
myDateTimeField.ParseAsDateTime()
myObjectIDField := NewQField("myObjectID")
myObjectIDField.UseAliases("_id", "id") // will use _id and id to refer to myObjectID
myObjectIDField.ParseAsObjectID()
myMetaField := NewQField("pageMarker")
myMetaField.ParseAsMeta()
// create a new query processor
qproc := NewQProcessor(myStringField, myStringFieldWithDefault, myIntField, myFloatField, myBoolField, myDateTimeField, myObjectIDField, myMetaField)

// we'll use the net/url package's Values to construct query to process, but it would be more common to use one from an http request
qs := url.Values{}
qs.Add("unknown", "nin:1,2,3,4") // a QField was not created for 'unknown' so it will be ignored
qs.Add("myString", "like:Hello, world")
qs.Add("myInt", "gt:1,lt:10")
qs.Add("myFloat", "1.0") // 'eq:' operator is assumed
qs.Add("myBool", "false") // 'eq:' operator is assumed
qs.Add("myDateTime", "gte:2021-01-01T15:00:00Z,lte:2021-02-01T15:00:00Z")
qs.Add("id", "in:6050e7f529a90b22dc47f19e,6050e7f529a90b22dc47f19f") // using an alias of myObjectID
qs.Add("pageMarker", "6050e7f529a90b22dc47f19f")
qs.Add("srt", "-myInt,+myString") // sort by myInt in descending order (myStringField.Sortable() was not called so +myString will be ignored)
qs.Add("prj", "-myFloat") // exclude myFloat field from query results
qs.Add("lmt", "10") // limit to 10 results
qs.Add("skp", "100") // skip the first 100 results

result, err := qproc(qs)
// do something with result if err == nil
```

### JSON Output

```json
{
  "Filter": {
    "myBool": {
      "$eq": false
    },
    "myDateTime": {
      "$gte": "2021-01-01T15:00:00Z",
      "$lte": "2021-02-01T15:00:00Z"
    },
    "myFloat": {
      "$eq": 1
    },
    "myInt": {
      "$gt": 1,
      "$lt": 10
    },
    "myObjectID": {
      "$in": ["6050e7f529a90b22dc47f19e", "6050e7f529a90b22dc47f19f"]
    },
    "myString": {
      "$regex": "Hello, world",
      "$options": "i"
    },
    "myStringWithDefault": {
      "$regex": "^Something useful",
      "options": "i"
    }
  },
  "Projection": {
    "myFloat": 0
  },
  "Sort": {
    "myInt": -1
  },
  "Limit": 10,
  "Skip": 100,
  "Meta": {
    "pageMarker": "6050e7f529a90b22dc47f19f"
  }
}
```

## QField

Query fields (QField) are used to build query processors (QProcessor). It is recommended to use the _NewQueryField_ method when creating a new QField.

| Property       | Type            | Description                                                                                                                 |
| -------------- | --------------- | --------------------------------------------------------------------------------------------------------------------------- |
| Key            | string          | The key of the field in as it will appear in the query string. This should match the target field in the database schema.   |
| Type           | QType           | The type used when parsing query strings for this field                                                                     |
| Default        | \*func() string | An optional function that will be used to set the filter for this field if the field is missing or if the value is invalid. |
| Aliases        | []string        | A slice of strings that can be used as aliases for the field's key.                                                         |
| IsProjectable  | Bool            | Whether the field is allowed in projections or not.                                                                         |
| IsSortable     | Bool            | Whether the field is allowed to be used to sort or not.                                                                     |
| IsMeta         | Bool            | Whether the field is used as a meta field. See [Meta Fields](#meta-fields)                                                  |
| HasDefaultFunc | Bool            | Whether a Default function was set (call _UseDefaultFunc_ to set the Default function)                                      |

### Reserved Keys

When creating a QField, some values can not be used for the field key as they would conflict with the following built-in keys.

| Key | Description                                                                                         |
| --- | --------------------------------------------------------------------------------------------------- |
| lmt | Used to specify a query result limit                                                                |
| skp | Used to specify how many documents to skip in the query results                                     |
| srt | Used to specify one or more fields to sort by                                                       |
| prj | Used to specify which fields to include/exclude from the documents in the query result (projection) |

### Comparision Operators

| Operator | QType   | Description                                                       |
| -------- | ------- | ----------------------------------------------------------------- |
| eq:      | any     | Equal to - if no operator is detected the eq: operator is assumed |
| ne:      | any     | Not equal to                                                      |
| gt:      | any     | Greather than                                                     |
| lt:      | any     | Less than                                                         |
| gte:     | any     | Greater than or equal to                                          |
| lte:     | any     | Less than or equal to                                             |
| in:      | any     | Includes one or more values                                       |
| nin:     | any     | Does not include one or more values                               |
| all:     | any     | Contains all values                                               |
| like:    | QString | Contains a character sequence                                     |
| slike:   | QString | Starts with a character sequence                                  |
| elike:   | QString | Ends with a character sequence                                    |

### Sort Operators

| Operator | Description                                                            |
| -------- | ---------------------------------------------------------------------- |
| +        | Ascending order - if no operator is detected the + operator is assumed |
| -        | Descending order                                                       |

### Projection Operators

_NOTE:_ MongoDB does not support mixed include/exclude projections. The first operator found is used for all projection fields.

| Operator | Description                                                           |
| -------- | --------------------------------------------------------------------- |
| +        | Include field - if no opertator is detected the + operator is assumed |
| -        | Exclude field                                                         |

<h3 id="qfield-methods">Methods</h3>

All QField methods return \*QField so that the methods are chainable.

| Method          | Args          | Return Type | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| --------------- | ------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| UseDefault      | func() string | \*QField    | Sets the QField's Default function to run when the field is missing/is invalid in the query string. This also sets the _HasDefaultFunc_ property to `true`. If the field is to be parsed as _anything other than Meta_, the Default function must return a `string` using [MongoQS syntax](#syntax). Default functions for Meta fields _should not_ use MongoQS query string syntax as they will be parsed and validated by developers - see [Meta Fields](#meta-fields) for more info. |
| UseAliases      | ...string     | \*QField    | Adds one or more aliases to the QField allowing it query strings to refer to the field without using its name                                                                                                                                                                                                                                                                                                                                                                           |
| IsProjectable   |               | \*QField    | Allows the QField to be used in projections.                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| IsSortable      |               | \*QField    | Allows the QField to be used to sort.                                                                                                                                                                                                                                                                                                                                                                                                                                                   |
| ParseAsString   |               | \*QField    | Instructs the processor to parse the field values as a strings.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ParseAsInt      |               | \*QField    | Instructs the processor to parse the field values as an integers.                                                                                                                                                                                                                                                                                                                                                                                                                       |
| ParseAsFloat    |               | \*QField    | Instructs the processor to parse the field values as floating point numbers.                                                                                                                                                                                                                                                                                                                                                                                                            |
| ParseAsBool     |               | \*QField    | Instructs the processor to parse the field values as booleans.                                                                                                                                                                                                                                                                                                                                                                                                                          |
| ParseAsDateTime |               | \*QField    | Instructs the processor to parse the field values as datetimes.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ParseAsObjectID |               | \*QField    | Instructs the processor to parse the field values as ObjectIDs.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| ParseAsMeta     |               | \*QField    | Instructs the processor to parse the field value as a string and add it to the QResult Meta instead of thee QResult Filter.                                                                                                                                                                                                                                                                                                                                                             |

### About Meta Fields

Meta fields allow query parameters to be accepted by the processor but not added to the QResult Filter. The Meta values will appear in the QResult Meta property which is of type `map[string]string`. It is the developer's responsibility to parse and validate the Meta values in the QResult. Meta fields can be configured with aliases and a Default method.

Meta fields may be useful for allowing clients to specify options, like allowing the request to specify a `pageMarker` (or similar) which would likely be the ObjectID of the last document in a previous query that the request handler could then use to modify the QResult Filter to include an additional parameter that queries the collection appropriately.

## Query Strings

### Syntax

`<field>=<operator>:<value>,<value>`

### Equal To

`int=1`

`int=eq:1`

### Not Equal To

`int=ne:1`

### Greater Than, Less Than

`int=gt:1`

`int=lt:1`

### Greater Than Equal To, Less Than Equal To

`int=gte:1`

`int=lte:1`

### In, Not In, All

`int=in:1,2,3`

`int=nin:1,2,3`

`int=all:1,2,3`

### Like, Starts Like, Ends Like

`str=like:abc`

`str=slike:a`

`str=elike:bc`

### Mixed

`int=gt:1,lte:5,str=like:abc,srt=-int,lmt=10,skp=100,prj=str`

Find documents where `int` is greater than `1` and less than or equal to `5`; sort by `int` in descending order; limit the number of returned documents to `10`; skip the first `100` documents; only include `str` in the returned documents.

## QResult

| Property   | Type                | Default | Description                                          |
| ---------- | ------------------- | ------- | ---------------------------------------------------- |
| Filter     | bson.M              | {}      | MongoDB query filter                                 |
| Projection | bson.M              | {}      | MongoDB field projection                             |
| Sort       | bson.M              | {}      | MongoDB sort criteria                                |
| Limit      | int                 | 0       | The number of documents to limit the query result to |
| Skip       | int                 | 0       | The number of documents to skip in the query result  |
| Meta       | `map[string]string` | {}      | Key value pairs                                      |

## Backlog

- Nested wild card fields
  - Field names will be able to be defined as `field.*` or `field.*.nested` (not `field.*.*` though). This will allow querying nested document fields that may be dynamically set.
