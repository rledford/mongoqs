package mongoqs

import (
	"fmt"
	"net/url"
	"testing"
)

func TestNewQProcessor(t *testing.T) {
	// create and configure query fields
	myStringField := NewQField("myString", QString)
	myIntField := NewQField("myInt", QInt)
	myIntField.IsSortable() // allow this field to be used in sorts
	myIntField.IsProjectable() // // allow this field to be used in projections
	myFloatField := NewQField("myFloat", QFloat)
	myFloatField.IsSortable().IsProjectable() // same as calls on myIntField but chained
	myBoolField := NewQField("myBool", QBool)
	myDateTimeField := NewQField("myDateTime", QDateTime)
	myDateTimeField.UseTimeLayout("Mon Jan _1 00:00:00 2000", "2000-01-01T00:00:00Z") // will use ANSIC and RFC3339 layouts to parse times
	myObjectIDField := NewQField("myObjectID", QObjectID)
	myObjectIDField.UseAlias("_id", "id") // will use _id and id to refer to myObjectID
	// create a new query processor
	qproc := NewQProcessor(myStringField, myIntField, myFloatField, myBoolField, myDateTimeField, myObjectIDField)

	// we'll use the net/url package's Values to construct query to process, but it would be more common to use on from an http request
	qs := url.Values{}
	qs.Add("unknown", "nin:1,2,3,4") // a QField was not created for 'unknown' so it will be ignored
	qs.Add("myString", "in:a,b,c,d")
	qs.Add("myInt", "gt:1,lt:10")
	qs.Add("myFloat", "1.0") // 'eq:' operator is assumed
	qs.Add("myBool", "false") // 'eq:' operator is assumed
	qs.Add("myDateTime", "gte:2021-01-01T15:00:00Z")
	qs.Add("id", "6050e7f529a90b22dc47f19e") // using an alias of myObjectID
	qs.Add("srt", "-myInt,+myString") // sort by myInt in descending order (myStringField.IsSortable() was not called so +myString will be ignored)
	qs.Add("prj", "-myFloat") // exclude myFloat field from query results
	qs.Add("lmt", "10") // limit to 10 results
	qs.Add("skp", "100") // skip the first 100 results

	result, err := qproc(qs)
	if err == nil {
		fmt.Println(result.String())
	}
}