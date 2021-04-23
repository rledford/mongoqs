package mongoqs

import (
	"fmt"
	"log"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// comparison operators
const eq string = "eq:" // equal to
const ne string = "ne:" // not equal to
const gt string = "gt:" // greater than
const gte string = "gte:" // greater than or equal to
const lt string = "lt:" // less than
const lte string = "lte:" // less than or equal to
const in string = "in:" // in list of values
const nin string = "nin:" // not in list of values
const all string = "all:" // has all in list of values

// sort operators
const asc string = "+" // ascending
const des string = "-" // decending

// projection operators
const inc string = "+" // include
const exc string = "-" // exclude

// search operators (string fields only)
const like string = "like:" // includes sequence
const slike string = "slike:" // starts with sequence
const elike string = "elike:" // ends with sequence

// reserved query fields
const lmt string = "lmt" // MongoDB query limit count
const skp string = "skp" // MongoDB query skip count
const srt string = "srt" // MongoDB query sort
const prj string = "prj" // MongoDB query projection

// qvalue op list
var oplist []string = []string{eq, ne, gt, gte, lt, lte, in, nin, all, like, slike, elike}
var opregex *regexp.Regexp = regexp.MustCompile(strings.Join(oplist, "|"))

// toMOp - Adds leading $ to the provided operator
func toMOp(op string) string {
	return "$" + op[0:len(op) - 1]
}

// toOpValueMap - Builds a map of operator keys to values
func toOpValueMap(qvalue string, t QType) map[string][]string {
	result := make(map[string][]string)
	opindexes := opregex.FindAllStringIndex(qvalue, len(qvalue))
	if len(opindexes) > 0 {
		if opindexes[0][0] > 0 {
			// operator not found at beginning of qvalue, assuming eq: up to first found operator
			result[eq] = append(result[eq], strings.Split(strings.TrimSuffix(qvalue[0:opindexes[0][0]], ","),",")...)
		}
		for i, oi := range opindexes {
			op := qvalue[oi[0]:oi[1]]
			if i + 1 < len(opindexes) {
				// get a slice of qvalue from the end of the operator to the beginning of the next operator - values split at ,
				endindex := opindexes[i+1][0]
				value := strings.Split(strings.TrimSuffix(qvalue[oi[1]:endindex], ","),",")
				result[op] = append(result[op], value...)
			} else {
				// get a slice from the end of the current operator to the end of the qvalue - values split at ,
				result[op] = append(result[op], strings.Split(strings.TrimSuffix(qvalue[oi[1]:], ","),",")...)
			}
		}
	} else {
		// no operators found, assuming eq: for entire qvalue
		result[eq] = append(result[eq], strings.Split(strings.TrimSuffix(qvalue, ","),",")...)
	}

	return result
}

// QueryProcessorFn - function signature for a query processor
type QueryProcessorFn func(q url.Values) (QResult, error)

// QResult - Query result containing Filter, Limit, Skip, Sort, and Projection parameters compatible with MongoDB.
type QResult struct {
	Filter bson.M // MongoDB filter
	Projection bson.M // MongoDB projection
	Limit int64 // MongoDB document limit
	Skip int64 // MongoDB ocument skip count
	Sort bson.M // MongoDB sort
	Meta map[string]string // Map of keys to raw qstring value
}
func (r *QResult) String() string {
	return fmt.Sprintf(`
	----- Filter -----
	%v
	------------------
	--- Projection ---
	%v
	------------------
	------ Sort ------
	%v
	------------------
	----- Paging -----
	Limit:  %d
	Skip:   %d
	------------------
	------ Meta ------
	%v
	------------------
	` , r.Filter, r.Projection, r.Sort, r.Limit, r.Skip, r.Meta)
}

type QType int
// QString - Allows query values to be processed as strings. Does not apply to QResult if the value is empty after removing leading and trailing white space.
const QString QType = 0 // QField created without setting Type will default to string
// QInt - Allows query values to be processed as integers. Does not apply to QResult if parsing fails.
const QInt QType = 1
// QFloat - Allows query values to be processed as floating point numbers. Does not apply to QResult if parsing fails.
const QFloat QType = 2
// QBool - Allows query values to be processed as booleans. Does not apply to QResult if parsing fails.
const QBool QType = 3
// QDateTime - Allows query values to be processed as datetimes using formats added with the UseTimeLayout method. If one or more formats are not provided then time.RFC3339 is used. Does not apply to QResult if the date is invalid.
const QDateTime QType = 4
// QObjectID - Allows query values to be processed as MongoDB ObjectIDs. Does not apply to QResult if the value is not a valid ObjectID.
const QObjectID QType = 5

// QField - Query field definition. Key and Aliases cannot be empty or use any of the following reserved values: 'qlmt', 'qskp', 'qsrt', 'qprj'. If provided, the Default method should return a valid MongoDB filter parameter.
type QField struct {
	Type QType // The data type expected when parsing the values of query parameter values
	Key string // The target parameter in the request query string - supports dot notation for nested fields
	Default func() string // Function to run if this field is missing/is invalid - the result should be a string that the processor will parse into it's appropriate type for non-Meta fields
	Aliases []string // List of aliases that can be used as alternatives to this QField.Key
	IsProjectable bool // If true, this QField may be used for projections
	IsSortable bool // If true, this QField can be used for sorting
	IsMeta bool // If true, this QFieeld will be used as a meta field
	HasDefaultFunc bool // If true, the Default function will be used if a the field is missing/is invalid
}
// ApplyFilter - Processes the qvalue as the specified Type and applies the result to the provided out QResult.
func (f *QField) ApplyFilter(qvalue string, out *QResult) {
	opValueMap := toOpValueMap(qvalue, f.Type)
	result := bson.M{}
	nfilters := 0
	for op, values := range opValueMap {
		switch op {
		case eq, ne, gt, gte, lt, lte:
			if f.Type == QString {
				nfilters++
				// rejoin split values to use literal qvalue in query
				result[toMOp(op)] = strings.Join(values, ",")
				continue
			}
			for _, v := range values {
				switch f.Type {
				case QInt:
					i, err := strconv.ParseInt(v, 10, 64)
					if err == nil {
						nfilters++
						result[toMOp(op)] = i
					}
				case QFloat:
					flt, err := strconv.ParseFloat(v, 64)
					if err == nil {
						nfilters++
						result[toMOp(op)] = flt
					}
				case QBool:
					b, err := strconv.ParseBool(v)
					if err == nil {
						nfilters++
						result[toMOp(op)] = b
					}
				case QDateTime:
					d, err := time.Parse(time.RFC3339, v)
					if err == nil {
						nfilters++
						result[toMOp(op)] = primitive.NewDateTimeFromTime(d)
					}
				case QObjectID:
					id, err := primitive.ObjectIDFromHex(v)
					if err == nil {
						nfilters++
						result[toMOp(op)] = id
					}
				}
			}
		case in, nin, all:
			switch f.Type {
			case QString:
				nfilters++
				result[toMOp(op)] = values
			case QInt:
				vlist := []int64{}
				for _, v := range values {
					i, err := strconv.ParseInt(v, 10, 64)
					if err == nil {
						vlist = append(vlist, i)
					}
				}
				if len(vlist) > 0 {
					nfilters++
					result[toMOp(op)] = vlist
				}
			case QFloat:
				vlist := []float64{}
				for _, v := range values {
					flt, err := strconv.ParseFloat(v, 64)
					if err == nil {
						vlist = append(vlist, flt)
					}
				}
				if len(vlist) > 0 {
					nfilters++
					result[toMOp(op)] = vlist
				}
			case QBool:
				vlist := []bool{}
				for _, v := range values {
					b, err := strconv.ParseBool(v)
					if err == nil {
						vlist = append(vlist, b)
					}
				}
				if len(vlist) > 0 {
					nfilters++
					result[toMOp(op)] = vlist
				}
			case QDateTime:
				vlist := []primitive.DateTime{}
				for _, v := range values {
					d, err := time.Parse(time.RFC3339, v)
					if err == nil {
						vlist = append(vlist, primitive.NewDateTimeFromTime(d))
					}
				}
				if len(vlist) > 0 {
					nfilters++
					result[toMOp(op)] = vlist
				}
			case QObjectID:
				vlist := []primitive.ObjectID{}
				for _, v := range values {
					id, err := primitive.ObjectIDFromHex(v)
					if err == nil {
						vlist = append(vlist, id)
					}
				}
				if len(vlist) > 0 {
					nfilters++
					result[toMOp(op)] = vlist
				}
			}
		case like:
			switch f.Type {
			case QString:
				nfilters++
				result["$regex"] = regexp.QuoteMeta(strings.Join(values, ","))
				result["$options"] = "i"
			}
		case slike:
			switch f.Type {
			case QString:
				nfilters++
				result["$regex"] = "^" + regexp.QuoteMeta(strings.Join(values, ","))
				result["$options"] = "i"
			}
		case elike:
			switch f.Type {
			case QString:
				nfilters++
				result["$regex"] = regexp.QuoteMeta(strings.Join(values, ",")) + "$"
				result["$options"] = "i"
			}
		}
	}
	
	if nfilters > 0 {
		out.Filter[f.Key] = result
	}
}
// UseDefault - Sets the Default method to the provided function. Returns caller for chaining.
func (f *QField) UseDefault(fn func() string) *QField{
	f.Default = fn
	f.HasDefaultFunc = true
	return f
}

// UseAliases - Adds one or more aliases to this field. Returns caller for chaining.
func (f *QField) UseAliases(alias ...string) *QField {
	f.Aliases = append(f.Aliases, alias...)
	return f
}
// Projectable - Allows field to be used in projections. Returns caller for chaining.
func (f *QField) Projectable() *QField{
	f.IsProjectable = true
	return f
}
// Sortable - Allows field to be used in sorts. Returns caller for chaining.
func (f *QField) Sortable() *QField {
	f.IsSortable = true
	return f
}

// ParseAsMeta - Indicates that this field will not appear in the QResult Filter and will be parsed/interpreted outside of MongoQS
func (f *QField) ParseAsMeta() *QField {
	f.Type = QString
	f.IsMeta = true
	return f
}
// ParseAsString - Indicates that this field represents a database document field that contains a string value
func (f *QField) ParseAsString() *QField {
	f.Type = QString
	return f
}
// ParseAsInt - Indicates that this field represents a database document field that contains an integer value
func (f *QField) ParseAsInt() *QField {
	f.Type = QInt
	return f
}
// ParseAsFloat - Indicates that this field represents a database document field that contains a floating point number value
func (f *QField) ParseAsFloat() *QField {
	f.Type = QFloat
	return f
}
// ParseAsBool - Indicates that this field represents a database document field that contains a boolean value
func (f *QField) ParseAsBool() *QField {
	f.Type = QBool
	return f
}
// ParseAsDateTime - Indicates that this field represents a database document field that contains a datetime value
func (f *QField) ParseAsDateTime() *QField {
	f.Type = QDateTime
	return f
}
// ParseAsObjectID - Indicates that this field represents a database document field that contains a string value
func (f *QField) ParseAsObjectID() *QField {
	f.Type = QObjectID
	return f
}

// NewQField - Returns a new Qfield with the provided key and type.
func NewQField(key string) QField {
	return QField{Key: key}
}

// NewQResult - Returns a new empty QResult. Should be passed as the *out parameter when calling the processor function returned from NewRequestQueryProcessor.
func NewQResult() QResult {
	result := QResult{}
	result.Filter = bson.M{}
	result.Projection = bson.M{}
	result.Sort = bson.M{}
	result.Meta = make(map[string]string)

	return result
}

// NewQProcessor - Validates the provided QFields and returns a function that converts a URL query to a QResult.
func NewQProcessor(fields ...QField) QueryProcessorFn {
	// validate fields to ensures each field's Key and Aliases are not empty or using reserved values
	for _, f := range fields {
		switch f.Key {
		case "":
			log.Fatal(fmt.Sprintf("Field %q cannot be an empty string\n", f.Key))
		case lmt, skp, srt, prj:
			log.Fatal(fmt.Sprintf("Field %q is using a reserved key - reserved keys: %q, %q, %q, %q\n", f.Key, lmt, skp, srt, prj))
		}
		for _, a := range f.Aliases {
			switch a {
			case "":
				log.Fatal(fmt.Sprintf("Field %q alias cannot be an empty string\n", f.Key))
			case lmt, skp, srt, prj:
				log.Fatal(fmt.Sprintf("Field %q alias %q is using a reserved key - reserved keys: %q, %q, %q, %q\n", f.Key, a, lmt, skp, srt, prj))
			}
		}
		if f.IsMeta {
			if f.Type != QString {
				// Although meta fields are not processed the same as other fields, and having the Type set to something other than QString will not break the processor,
				// developers should be told they are attempting to do something that will not work as expected since meta fields will always be parsed as QString
				log.Fatal(fmt.Sprintf("Field %q is a meta field and can only be parsed as type QString", f.Key))
			}
			if f.IsSortable || f.IsProjectable {
				// Although meta fields are not processed the same as other fields, and having the IsProjectable and IsSortable flags set will not break the processor,
				// developers should be told they are attempting to do something that will not work as expected since meta fields will never appear in the QResult Sort property
				log.Fatal(fmt.Sprintf("Field %q is a meta field and will never appear in Projection or Sort - modify %q to not be projectable or sortable\n", f.Key, f.Key))
			}
		}
	}
	return func(query url.Values) (QResult, error) {
		result := NewQResult()
		projections := make(map[string]int)
		projsum := 1 // incremented or decremented with each +/- operator found on a qprj qvalue. normalized to 0 or 1 after summing the operators
		sorts := make(map[string]int)
		// map projections and sum
		for _, proj := range strings.Split(query.Get(prj), ",") {
			if len(proj) == 0 {
				continue
			}
			if strings.HasPrefix(proj, inc) {
				projections[proj[1:]] = 1
				projsum++
			} else if strings.HasPrefix(proj, exc) {
				projections[proj[1:]] = -1
				projsum--
			} else {
				projections[proj] = 1
				projsum++
			}
		}
		// normalize projsum to 0 or 1
		projsum = int(math.Max(0, math.Min(1, float64(projsum))))

		// map sorts
		for _, sort := range strings.Split(query.Get(srt), ",") {
			if len(sort) == 0 {
				continue
			}
			if strings.HasPrefix(sort, asc) {
				sorts[sort[1:]] = 1
			} else if strings.HasPrefix(sort, des) {
				sorts[sort[1:]] = -1
			} else {
				sorts[sort] = 1
			}
		}

		// apply limit
		if l, err := strconv.ParseInt(query.Get(lmt), 10, 64); err == nil {
			result.Limit = l
		}
		// apply skip
		if s, err := strconv.ParseInt(query.Get(skp), 10, 64); err == nil {
			result.Skip = s
		}

		// process fields
		for _, field := range fields {
			qvalue := query.Get(field.Key)
			// search for applicable alias if field is not found by key
			if qvalue == "" {
				for _, a := range field.Aliases {
					qvalue = query.Get(a)
					if qvalue != "" {
						// alias found - break loop
						break
					}
				}
			}
			if qvalue == "" && field.HasDefaultFunc {
				qvalue = field.Default()
			}
			if qvalue == "" {
				// skip to next field since no qvalue was found so it doesn't appear in the Filter at all
				continue
			}
			if field.IsMeta {
				result.Meta[field.Key] = qvalue
				// skip further logic as meta fields should not be used in projections, sorts, or filters
				continue
			}
			// apply projections
			if field.IsProjectable {
				if _, ok := projections[field.Key]; ok {
					result.Projection[field.Key] = projsum
				} else {
					for _, alias := range field.Aliases {
						if _, ok := projections[alias]; ok {
							result.Projection[field.Key] = projsum
						}
					}
				}
			}
			// apply sorts
			if field.IsSortable {
				if ord, ok := sorts[field.Key]; ok {
					result.Sort[field.Key] = ord
				} else {
					for _, alias := range field.Aliases {
						if ord, ok := sorts[alias]; ok {
							result.Sort[field.Key] = ord
						}
					}
				}
			}
			// apply filter
			field.ApplyFilter(qvalue, &result)
		}

		return result, nil
	}
}