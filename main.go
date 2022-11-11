// Package url implements utilities for parsing url.Values into an organized and structured map.
//
// Each key is parsed progressively and organized depending on the value type it represents.
//
//	form[]=A&form[]=B&form[]=C // a ValueSlice of three elements 0 to 2
//	form[0]=A&form[2]=B&form[3]=C // a ValueSlice of three elements 0 to 3, element 1 will be ValueNil.
//	                              // when a slice index is defined, only the first item of the url.Value is parsed
//	form[4][key][subKey]=A // a ValueMap, "key" a ValueMap, "subKey" a ValueString
//
// ParseValues() creates a Map, Map.Get(...any) descends through key/values pairs
//
//	mapV := url.ParseValues(url.Values)
//	mapV.Get("form") // ValueSlice
//	mapV.Get("form", 0) // ValueString
//	mapV.Get("form", 1) // ValueNil
//	mapV.Get("form", 2) // ValueString
//	mapV.Get("form", 3) // ValueString
//	mapV.Get("form", 4, "key") // ValueMap
//	mapV.Get("form", 4, "key", "subKey") // ValueString
//
// Inspired by PHP's $_GET, $_POST and $_REQUEST.
//
// # About the parser
//
// When cycling url.Values, keys should be returned in the order they were submitted by the browser/client,
// however when using range on url.Values key order may differ for identical request payloads.
//
// This behavior makes it hard to handle array values, especially when they are root for hash values.
// More information can be found in https://github.com/golang/go/issues/29985 Golang Issue
//
// Consider the input below:
//
//	key[]=A & key[1]=C & key[2]=C
//
// If parsed in the order above it should result in
//
//	[ 0 => A, 1 => B, 2 => C ]
//
// If parsed instead as
//
//	key[2]=C & key[]=A & key[1]=B
//
// It would result in:
//
//	[ 0 => Nil, 1 => B, 2 => C, 3 => A ]
//
// To provide some predictability to ParseValue() keys are sorted by name:
//
//	sort.Strings(keys)
//
// Below some pivotal points I believe you should consider when creating Form/QueryString.
// In the examples below {} indicates an hash map, [] indicates an array.
//
// 1. Array key index should be defined, the package queues values if no index is defined.
//
//	"input[][key1] = A"
//	"input[][key2] = B"
//
// Will result in:
//
//	"{ input : [ 0 => { key1 : A }, 1 => { key2 : B } ] }"
//
// 2. The behavior when array values have mixed "[]" and "[%d]" format is not guaranteed to report all values (see Google Issue).
// If the key is already defined the value will be replaced and since "key[]" will append values, "key[]" stands for "key[ len(array) ]"
//
//	"input[0] = A"
//	"input[1] = B"
//	"input[] = C"
//	"input[] = D"
//
// is parsed as
//
//	"{ input : [ 0 => A, 1 => B, 2 => C, 3 => D ] } "
//
// Scenarios like:
//
//	"input[] = C"
//	"input[0] = A"
//	"input[] = D"
//	"input[1] = B"
//
// is parsed as
//
//	"{ input : [ 0 => A, 1 => B ] } " // as input[] = C and input[] = D will be replaced by input[0] = A and input[1] = B
//
// 3. When an array key is missing, missing keys will be filled with ValueNil typed Value. So that:
//
//	"input[3] = A"
//	"input[1] = B"
//	"input[2] = C"
//	"input[5] = D"
//
// is parsed as
//
//	"{ input : [ 0 => nil, 1 => B, 2 => C, 3 => A, 4 => nil, 5 => D ] }"
//
// 4. When an element has been parsed as a Map or Slice, subsequent Keys must respect the same type
//
//	"input[0] = a"
//	"input[key] = b"
//	"input[2] = c"
//
// depending on which value is parsed first, could be
//
//		"{ input : [ 0 => , 2 => c ] }"
//	 // or
//		"{ input : { key: b } }"
//
// 5. Malformed Key/Value pairs are ignored
package url

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

var ErrValueNotSlice = errors.New("Value is not a slice")
var ErrValueNotMap = errors.New("Value is not a map")
var ErrValueNotMapOrSlice = errors.New("Value is not a map or slice")

// sorts url.Values by key
func sortUrlValues(src url.Values) (keys []string) {
	keys = make([]string, len(src))
	i := 0
	for k := range src {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

// splits key name from possible brackets
func getParseKey(key string) (root string, nestedKeys []string, err error) {
	root = key
	strKeys := ""
	if strings.Contains(root, "[") && strings.Contains(root, "]") {
		root = key[:strings.Index(root, "[")]
		strKeys = key[len(root):]
	}
	nestedKeys = make([]string, strings.Count(strKeys, "["))
	i := 0
	for strings.Contains(strKeys, "[") {
		var start, end = strings.Index(strKeys, "["), strings.Index(strKeys, "]")
		if start == -1 || end == -1 {
			err = errors.New("malformed key")
			return
		}
		nestedKeys[i] = strKeys[start+1 : end]
		strKeys = strKeys[end+1:]
		i++
	}
	return
}

// Parse processes url.Values and returns a Map interface
//
// Parse does its best to aggregate and organize keys in url.Values.
// For example the input below:
//
//	"input4[key0] = value4[key0]"
//	"input4[key1][subKey1] = value4[key1][subKey1]"
//	"input4[key1][subKey2] = value4[key1][subKey2]"
//	"input4[key2][0] = value4[key2][0]"
//	"input4[key2][1] = value4[key2][1]"
//
// Will be interpreted as
//
//	{ "input4" :
//	  { "key0": "value4[key0]",
//	    "key1": { "subKey1": "value4[key1][subKey1]",
//	              "subKey2": "value4[key1][subKey2]" }
//	    "key2": [ "value4[key2][0]", "value4[key2][1]"]
//	  }
//	}
func ParseValues(src url.Values) (m Map, err error) {
	var value, nestedKeys []string
	var root string
	var currentValue Value
	var sIndex int = -1

	out := newNilValue("").to(ValueMap)
keyLoop:
	for _, key := range sortUrlValues(src) {
		value = src[key]
		if root, nestedKeys, err = getParseKey(key); err != nil {
			// missing [ or ]
			// malformed input, ignoring value
			continue keyLoop
		}

		var appendSlice = false
		currentValue, _ = out.mapFor(root)
		previousValue := currentValue
		for _, keyPart := range nestedKeys {
			appendSlice = false
			previousValue = currentValue
			if keyPart == "" {
				appendSlice = true
				currentValue = currentValue.to(ValueSlice)
				// currentValue is a slice, but if the code reaches here, the next value will be a map.
				// creates a slice element to host the new map element.
				if currentValue, err = currentValue.newNilValueAt(-1); err != nil {
					// cannot set slice value for keyPart
					// malformed input, ignoring value
					continue keyLoop
				}
			} else if sIndex, err = strconv.Atoi(keyPart); err == nil {
				if !currentValue.cast(ValueSlice) {
					// keyPart is an integer, so it is expected to be either nil or slice.
					// malformed input, ignoring value
					continue keyLoop
				}
				if currentValue, err = currentValue.to(ValueSlice).newNilValueAt(sIndex); err != nil {
					// cannot set slice value for keyPart
					// malformed input, ignoring value
					continue keyLoop
				}
			} else if currentValue, err = currentValue.mapFor(keyPart); err != nil {
				// cannot set map key value for keyPart
				// malformed input, ignoring value
				continue keyLoop
			}
		}

		if appendSlice && previousValue.Is(ValueSlice) && len(value) > 1 {
			currentValue.to(ValueString).setValue(value[0])
			// value is a slice, creates elements
			appendStrings(previousValue, -1, value[1:]...)
			continue
		}

		if !currentValue.cast(ValueString) {
			// cannot cast the current value
			// malformed input, ignoring value
			continue keyLoop
		}

		currentValue.setValue(value[0])
	}
	return out, err
}

type IterValue func(Value) error

type Map interface {
	// Get descends into the map following the keys in order.
	// If an error occurs it returns ValueNil and the parsing error.
	//
	// keys can only be string or int
	//
	// Below some examples
	//  v, err := mapV.Get("input4")
	//  if v.Is(ValueMap) { ... }
	//
	//  v, err := mapV.Get("input4", "key0") // "input4[key0] = value4[key0]"
	//  if v.Is(ValueString) { fmt.Printf("%s => %s", v.Key(), v.String()) }
	//
	//  v, err := mapV.Get("input4", "key1", "subKey1") // "input4[key1][subKey1] = value4[key1][subKey1]"
	//  if v.Is(ValueString) { fmt.Printf("%s => %s", v.Key(), v.String()) }
	//
	//  v, err := mapV.Get("input4", "key1", "subKey2") // "input4[key1][subKey2] = value4[key1][subKey2]"
	//  if v.Is(ValueString) { fmt.Printf("%s => %s", v.Key(), v.String()) }
	//
	//  v, err := mapV.Get("input4", "key2") // "input4[key2][0] = value4[key2][0]"
	//  if v.Is(ValueSlice) { ... }
	//
	//  v, err := mapV.Get("input4", "key2", 0) // "input4[key2][0] = value4[key2][0]"
	//  if v.Is(ValueString) { fmt.Printf("%s => %s", v.Key(), v.String()) }
	//
	//  v, err := mapV.Get("input4", "key2", 1) // "input4[key2][1] = value4[key2][1]"
	//  if v.Is(ValueString) { fmt.Printf("%s => %s", v.Key(), v.String()) }
	//
	GetValue(...any) (Value, error)
	// GetString() behaves the same as Get(), ignore errors and returns "" if the value is not a string
	// Note that GetString() will return "" also if the keys are correct but the string is an empty string.
	//  mapV.Get("a", "b") // returns "" if a.b is not a string or an empty string.
	GetString(keys ...any) string
	// GetStrings() returns the content of a ValueSlice as a string slice.
	// If the value is not a type ValueSlice, returns an empty slice. Any non-string values are ignored.
	GetStrings(keys ...any) []string
	// Returns a map containing Key/Values pair.
	// Keys for array values are explicitly defined.
	//  {
	//  "input1": "value1",
	//  "input2": "value2",
	//  "input3[0]": "value3[0]",
	//  "input3[1]": "value3[1]",
	//  "input3[2]": "value3[2]",
	//  "input3[3]": "value3[]",
	//  "input3[4]": "value3[]"
	//  }
	KeyValue() map[string]string
	// Iterate each key in a ValueMap or ValueSlice.
	// When called on ValueNil and ValueString, it does nothing but returns ErrValueNotMapOrSlice.
	// When descending the keys Value.Key() returns the key relative to the position in the map.
	// If each func(Value) error returns a non-nil value, Each() stops descending that path.
	Each(each IterValue) error
}

type valueWriter interface {
	setValue(any)
	to(ValueType) Value
	newNilValueAt(int) (Value, error)
	newStringValueAt(int, string) (Value, error)
	mapFor(string) (Value, error)
	cast(t ValueType) bool
}

type Value interface {
	valueWriter
	Map
	// returns the value as a string, fails if Type() != ValueString
	String() (value string, ok bool)
	// returns the value as a []Value, fails if Type() != ValueSlice
	Slice() (value []Value, ok bool)
	// returns the value as a map[string]Value, fails if Type() != ValueMap
	Map() (value map[string]Value, ok bool)
	// key of the item in the Map
	Key() string
	// type of the value
	Type() ValueType
	// true if Type() matches t
	Is(t ValueType) bool
	// shortcut to Value.Is(ValueNil)
	IsNil() bool
	// ValueString returns the length of the string.
	// ValueSlice returns the length of the slice.
	// ValueMap returns the length of the map.
	Len() int
}

type ValueType int

const (
	// empty element
	ValueNil ValueType = iota
	// map value
	ValueMap
	// slice value
	ValueSlice
	// string value
	ValueString
)

func (vt ValueType) String() string {
	switch vt {
	case 1:
		return "ValueMap"
	case 2:
		return "ValueSlice"
	case 3:
		return "ValueString"
	}
	return "ValueNil"
}

type item struct {
	key       string
	value     any
	valueType ValueType
}

func (val *item) setValue(v any) {
	val.value = v
}

func (val *item) cast(t ValueType) bool {
	if val.Is(ValueNil) {
		val.to(t)
		return true
	} else if val.Is(t) {
		return true
	}
	return false
}

func (val *item) to(t ValueType) Value {
	if t == ValueString {
		if !val.Is(ValueString) {
			val.valueType = ValueString
			val.value = ""
		}
	}
	if t == ValueSlice {
		if !val.Is(ValueSlice) {
			val.valueType = ValueSlice
			var slice = make([]Value, 0, 10)
			val.value = &slice
		}
	}
	if t == ValueMap {
		if !val.Is(ValueMap) {
			val.valueType = ValueMap
			var mapV = make(map[string]Value, 0)
			val.value = &mapV
		}
	}
	return val
}

func (val *item) mapFor(k string) (Value, error) {
	if !val.cast(ValueMap) {
		return nil, ErrValueNotMap
	}
	valueKey := ""
	if val.key == "" {
		valueKey = k
	} else {
		valueKey = val.key + "[" + k + "]"
	}
	m := (val.value).(*map[string]Value)
	if s, ok := (*m)[k]; ok {
		return s, nil
	} else {
		(*m)[k] = newNilValue(valueKey)
	}
	return (*m)[k], nil
}

func appendStrings(val Value, sliceIndex int, list ...string) error {
	if !val.Is(ValueSlice) {
		return ErrValueNotSlice
	}
	for _, str := range list {
		nested, _ := val.newNilValueAt(sliceIndex)
		nested.to(ValueString).setValue(str)
	}
	return nil
}

func (val *item) newNilValueAt(sliceIndex int) (Value, error) {
	if !val.Is(ValueSlice) {
		return nil, ErrValueNotSlice
	}
	slice, _ := (val.value).(*[]Value)
	if sliceIndex == -1 {
		sliceIndex = len(*slice)
	}
	for i := 0; sliceIndex >= len(*slice); i++ {
		k := fmt.Sprintf("%s[%d]", val.key, len(*slice))
		*slice = append(*slice, newNilValue(k))
	}
	return (*slice)[sliceIndex], nil
}

func (val *item) newStringValueAt(sliceIndex int, value string) (v Value, err error) {
	if v, err = val.newNilValueAt(sliceIndex); err != nil {
		return nil, err
	}
	return v, err
}

func newNilValue(key string) *item {
	return &item{
		key:       key,
		value:     "",
		valueType: ValueNil,
	}
}

func (val *item) GetValue(keys ...any) (out Value, err error) {
	if len(keys) == 0 {
		return val, nil
	}
	out = val
	path := ""
	for kIndex, k := range keys {
		path += out.Key()
		if i, ok := k.(int); ok {
			if !out.Is(ValueSlice) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, Value not SliceValue found %s", path, kIndex, out.Type())
			}
			s, _ := out.Slice()
			if i >= len(s) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, index is out of range:%d", path, kIndex, i)
			}
			out = s[i]
		} else if s, ok := k.(string); ok {
			if !out.Is(ValueMap) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, Value is not a MapValue found %s", path, kIndex, out.Type())
			}
			m, _ := out.Map()
			if out, ok = m[s]; !ok {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, unknown key:%s", path, kIndex, s)
			}
		} else {
			return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, expected int|string found %T", path, kIndex, k)
		}
	}
	return
}

func (val *item) GetString(keys ...any) string {
	out, _ := val.GetValue(keys...)
	s, _ := out.String()
	return s
}

func (val *item) GetStrings(keys ...any) (slice []string) {
	out, _ := val.GetValue(keys...)
	if out.Is(ValueSlice) {
		in, _ := out.Slice()
		slice = make([]string, out.Len())
		for i, v := range in {
			slice[i], _ = v.String()
		}
	}
	return
}

func (val *item) Each(each IterValue) error {
	if val.Is(ValueMap) {
		m, _ := (val.value).(*map[string]Value)
		for _, v := range *m {
			if err := each(v); err != nil {
				return err
			}
			if v.IsNil() || v.Is(ValueString) {
				continue
			}
			v.Each(each)
		}
		return nil
	} else if val.Is(ValueSlice) {
		s, _ := (val.value).(*[]Value)
		for _, v := range *s {
			if err := each(v); err != nil {
				return err
			}
			if v.IsNil() || v.Is(ValueString) {
				continue
			}
			v.Each(each)
		}
		return nil
	}
	return ErrValueNotMapOrSlice
}

func (val *item) KeyValue() (out map[string]string) {
	out = make(map[string]string)
	val.Each(func(v Value) error {
		if v.Is(ValueString) || v.Is(ValueNil) {
			out[v.Key()], _ = v.String()
		}
		return nil
	})
	return
}

func (val *item) String() (value string, ok bool) {
	if val.value == nil {
		return "", true
	}
	value, ok = val.value.(string)
	return
}

func (val *item) Key() (value string) {
	return val.key
}
func (val *item) Type() (t ValueType) {
	return val.valueType
}

func (val *item) Slice() (value []Value, ok bool) {
	ptr, ok := (val.value).(*[]Value)
	if ok {
		value = *ptr
	}
	return
}

func (val *item) IsNil() bool {
	return val.Is(ValueNil)
}

func (val *item) Map() (value map[string]Value, ok bool) {
	ptr, ok := (val.value).(*map[string]Value)
	if ok {
		value = *ptr
	}
	return

}

func (val *item) Is(t ValueType) bool {
	return val.valueType == t
}

func (val *item) Len() int {
	switch val.valueType {
	case ValueMap:
		v := (val.value).(*map[string]Value)
		return len(*v)
	case ValueSlice:
		v := (val.value).(*[]Value)
		return len(*v)
	case ValueString:
		v := (val.value).(string)
		return len(v)
	}
	return 0
}
