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
const eq string = "eq" // equal to
const ne string = "ne" // not equal to
const gt string = "gt" // greater than
const gte string = "gte" // greater than or equal to
const lt string = "lt" // less than
const lte string = "lte" // less than or equal to
const in string = "in" // in list of values
const nin string = "nin" // not in list of values
const all string = "all" // has all in list of values

// sort operators
const asc string = "+" // ascending
const des string = "-" // decending

// projection operators
const inc string = "+" // include
const exc string = "-" // exclude

// search operators (string fields only)
const like string = "like" // includes sequence
const slike string = "slike" // starts with sequence
const elike string = "elike" // ends with sequence

// reserved query fields
const lmt string = "lmt" // MongoDB query limit count
const skp string = "skp" // MongoDB query skip count
const srt string = "srt" // MongoDB query sort
const prj string = "prj" // MongoDB query projection

// toMOp - Adds leading $ to the provided operator
func toMOp(op string) string {
	return "$" + op
}

// toOpValueMap - Reduces a qvalue to a map of operator keys to a slice of the values to cast to their appropriate type
func toOpValueMap(qvalue string) map[string][]string {
	// TODO: needs to be fixed to support values that include ':' that aren't preceded by a valid operator (such as dates)
	result := make(map[string][]string)
	values := strings.Split(qvalue, ",")
	currentOp := eq
	for _, v := range values {
		parts := strings.Split(v, ":")
		if len(parts) > 1 {
			currentOp = parts[0]
			result[currentOp] = append(result[currentOp], parts[1:]...)
		} else {
			result[currentOp] = append(result[currentOp], parts[0])
		}
	}

	// fmt.Println(result)

	return result
}

// QResult - Query result containing Filter, Limit, Skip, Sort, and Projection parameters compatible with MongoDB.
type QResult struct {
	Filter bson.M // MongoDB filter
	Projection bson.M // MongoDB projection
	Limit int64 // MongoDB document limit
	Skip int64 // MongoDB ocument skip count
	Sort bson.M // MongoDB sort
}
func (r *QResult) String() string {
	return fmt.Sprintf("--- Filter ---\n%v\n--------------\n--- Projection ---\n%v\n------------------\n--- Sort ---\n%v\n-------------\n--- Paging ---\nLimit:\t%d\nSkip:\t%d\n--------------", r.Filter, r.Projection, r.Sort, r.Limit, r.Skip)
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

// QField - Query field definition. Name and Aliases cannot be empty or use any of the following reserved values: 'qlmt', 'qskp', 'qsrt', 'qprj'. If provided, the Default method should return a valid MongoDB filter parameter.
type QField struct {
	Type QType // The data type expected when parsing the values of query parameter values
	Name string // The target parameter in the request query string - supports dot notation for nested fields
	Default *func() bson.M // Pointer to a function to run if this field is missing/is invalid
	Validators []*func() error // Pointer to function to run to validate the field
	Aliases []string // List of aliases that can be used as alternatives to this QField.Name
	Projectable bool // If true, this QField may be used for projections
	Sortable bool // If true, this QField can be used for sorting
	TimeLayouts []string // Date parsing formats
}
// ApplyFilter - Processes the qvalue as the specified Type and applies the result to the provided out QResult.
func (f *QField) ApplyFilter(qvalue string, out *QResult) {
	opValueMap := toOpValueMap(qvalue)
	result := bson.M{}
	nfilters := 0
	for op, values := range opValueMap {
		switch op {
		case eq, ne, gt, gte, lt, lte:
			for _, v := range values {
				switch f.Type {
				case QString:
					nfilters++
					result[toMOp(op)] = v
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
					
					if len(f.TimeLayouts) > 0 {
						for _, format := range f.TimeLayouts {
							d, err := time.Parse(format, v)
							if err == nil {
								nfilters++
								result[toMOp(op)] = d
								break;
							}
						}
					} else {
						d, err := time.Parse(time.RFC3339, v)
						if err == nil {
							nfilters++
							result[toMOp(op)] = primitive.NewDateTimeFromTime(d)
						}
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
					if len(f.TimeLayouts) > 0 {
						for _, format := range f.TimeLayouts {
							d, err := time.Parse(format, v)
							if err == nil {
								vlist = append(vlist, primitive.NewDateTimeFromTime(d))
								break;
							}
						}
					} else {
						d, err := time.Parse(time.RFC3339, v)
						if err == nil {
							vlist = append(vlist, primitive.NewDateTimeFromTime(d))
						}
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
				result["$regex"] = primitive.Regex{
					Pattern: regexp.QuoteMeta(strings.Join(values, "")),
					Options: "i",
				}
			}
		case slike:
			switch f.Type {
			case QString:
				nfilters++
				result["$regex"] = primitive.Regex{
					Pattern: "^" + regexp.QuoteMeta(strings.Join(values, "")),
					Options: "i",
				}
			}
		case elike:
			switch f.Type {
			case QString:
				nfilters++
				result["$regex"] = primitive.Regex{
					Pattern: regexp.QuoteMeta(strings.Join(values, "")) + "$",
					Options: "i",
				}
			}
		}
	}
	
	if nfilters > 0 {
		out.Filter[f.Name] = result
	}
}
// UseDefault - Sets the Default method to the provided func pointer. Returns caller for chaining.
func (f *QField) UseDefault(fn *func() bson.M) *QField{
	f.Default = fn
	return f
}
// IsProjectable - Allows field to be used in projections. Returns caller for chaining.
func (f *QField) IsProjectable() *QField{
	f.Projectable = true
	return f
}
// IsSortable - Allows field to be used in sorts. Returns caller for chaining.
func (f *QField) IsSortable() *QField {
	f.Sortable = true
	return f
}
// UseAlias - Adds one or more aliases for this field. Returns caller for chaining.
func (f *QField) UseAlias(alias ...string) *QField {
	f.Aliases = append(f.Aliases, alias...)
	return f
}
// UseTimeLayout - Adds one or more datetime layouts to be used when the QField type is QDateTime. Returns caller for chaining.
func (f *QField) UseTimeLayout(dtfmt ...string) *QField {
	if f.Type != QDateTime {
		log.Fatal(fmt.Sprintf("Field %q must be type QDateTime to add datetime layouts", f.Name))
	}
	f.TimeLayouts = append(f.TimeLayouts, dtfmt...)

	return f
}

// NewQField - Returns a new Qfield with the provided name and type.
func NewQField(name string, t QType) QField {
	return QField{Name: name, Type: t}
}

// NewQResult - Returns a new empty QResult. Should be passed as the *out parameter when calling the processor function returned from NewRequestQueryProcessor.
func NewQResult() QResult {
	result := QResult{}
	result.Filter = bson.M{}
	result.Projection = bson.M{}
	result.Sort = bson.M{}

	return result
}

// NewQProcessor - Validates the provided QFields and returns a function that converts a URL query to a QResult.
func NewQProcessor(fields ...QField) func (u url.Values) (QResult, error) {
	// validate fields to ensures each field's Name and Aliases are not empty or using reserved values
	for _, f := range fields {
		switch f.Name {
		case "":
			log.Fatal(fmt.Sprintf("Field %q cannot be an empty string\n", f.Name))
		case lmt, skp, srt, prj:
			log.Fatal(fmt.Sprintf("Field %q is using a reserved name (e.g. %q, %q, %q, %q)\n", f.Name, lmt, skp, srt, prj))
		}
		for _, a := range f.Aliases {
			switch a {
			case "":
				log.Fatal(fmt.Sprintf("Field %q alias cannot be an empty string\n", f.Name))
			case lmt, skp, srt, prj:
				log.Fatal(fmt.Sprintf("Field %q alias %q is using a reserved name (e.g. %q, %q, %q, %q)\n", f.Name, a, lmt, skp, srt, prj))
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
			qvalue := query.Get(field.Name)
			// alias := ""
			// search for applicable alias if field is not found by name
			if qvalue == "" {
				for _, a := range field.Aliases {
					qvalue = query.Get(a)
					if qvalue != "" {
						// alias found - break loop
						// alias = a
						break
					}
				}
			}
			if qvalue == "" {
				// apply default if Default function was provided
				if field.Default != nil {
					result.Filter[field.Name] = (*field.Default)()
				}
				continue
			}
			// apply projections
			if field.Projectable {
				if _, ok := projections[field.Name]; ok {
					result.Projection[field.Name] = projsum
				} else {
					for _, alias := range field.Aliases {
						if _, ok := projections[alias]; ok {
							result.Projection[field.Name] = projsum
						}
					}
				}
			}
			// apply sorts
			if field.Sortable {
				if ord, ok := sorts[field.Name]; ok {
					result.Sort[field.Name] = ord
				} else {
					for _, alias := range field.Aliases {
						if ord, ok := sorts[alias]; ok {
							result.Sort[field.Name] = ord
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