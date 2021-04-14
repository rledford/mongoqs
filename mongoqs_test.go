package mongoqs

import (
	"fmt"
	"net/url"
	"testing"
)

func TestNewQProcessor(t *testing.T) {
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
	if err == nil {
		fmt.Println(result.String())
	}
}