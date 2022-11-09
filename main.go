// Package url implements utilities for parsing url.Values into a organized and structured map.
//
// Each key is parsed progressively interpreting the type of value it represents.
//
//	form[]=A&form[]=B&form[]=C // a ValueSlice of three elements 0 to 2
//	form[0]=A&form[2]=B&form[3]=C // a ValueSlice of three elements 0 to 3, element 1 will be ValueNil.
//	                              // when a slice index is defined, only the first item of the url.Value is parsed
//	form[4][key][subKey]=A // a ValueMap, "key" a ValueMap, "subKey" a ValueString
//
// Use ParseValues() to obtain a Map, then use Get(...any) to descend through key/values pairs
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
// Inspired by PHP $_GET, $_POST and $_REQUEST.
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
var errKeyTypeMismatch = func(key, keyType string) error {
	return fmt.Errorf("value[%v] mismatch, expected %v", key, keyType)
}

// Parse processes url.Values and returns a Map interface
//
// This function does its best to aggregate and organize the data parsing url.Values
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
//
// # Caveats
//
// When cycling url.Values with range keys should be returned in the order submit by the browser/client,
// however http.Request.Form() and http.Request.URL.Query() have different key order for identical request payloads.
// This behavior makes it hard to handle array values, especially when used as root for hash values.
// To provide output predictability Parse() sorts the keys by name when processing the url.Values using
//
//	sort.Strings(keys)
//
// 1. When hash values after an array value, array key index should be defined.
//
//	"input[][key1] = a"
//	"input[][key2] = b"
//
// expected but not guaranteed result
//
//	"{ input : [ 0 => { key1 : a }, 1 => { key2 : b } ] }"
//
// 2. The behavior when array values have mixed "[]" and "[%d]" format is not guaranteed report all values.
//
//	"input[0] = a"
//	"input[1] = b"
//	"input[] = c"
//	"input[] = d"
//
// expected but not guaranteed result
//
//	"{ input : [ 0 => a, 1 => b, 2 => c, 3 => e ] } "
//
// 3. ValueNil typed Values could be in a ValueSlice typed Value if the indexes were missing or as a result of the previous comment.
//
//	"input[3] = a"
//	"input[1] = b"
//	"input[2] = c"
//	"input[5] = d"
//
// result would be
//
//	"{ input : [ 0 => , 1 => b, 2 => c, 3 => a, 4 => , 5 => d ] }"
//
// 4. When an element has been identified as a Map or Slice, subsequent Keys must respect the type.
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
func ParseValues(src url.Values) (m Map, err error) {
	out := &item{
		key:       "",
		value:     make(map[string]Value, 0),
		valueType: ValueMap,
	}
	// sorts url.Values by key
	keys := make([]string, 0)
	values := make(map[string][]string, 0)
	for k, v := range src {
		keys = append([]string{k}, keys...)
		values[k] = v
	}
	sort.Strings(keys)
keyLoop:
	for _, key := range keys {
		value := values[key]
		root := key
		strKeys := ""
		// splits key name from possible brackets
		if strings.Contains(root, "[") && strings.Contains(root, "]") {
			root = key[:strings.Index(root, "[")]
			strKeys = key[len(root):]
		}
		if strKeys == "" {
			// no brackets after the key name, simple string
			s, _ := out.setString(key)
			s.setValue(value[0])
			continue
		}
		var err error
		var sliceIndex int
		// currentValue
		currentValue, _ := out.mapFor(root)
		for strings.Contains(strKeys, "[") {
			var start, end = strings.Index(strKeys, "["), strings.Index(strKeys, "]")
			if start == -1 || end == -1 {
				// missing [ or ]
				// malformed input, ignoring value
				continue keyLoop
			}
			keyPart := strKeys[start+1 : end]
			strKeys = strKeys[end+1:]
			if keyPart == "" {
				sliceIndex = -1
				currentValue = currentValue.to(ValueSlice)
				continue
			} else if sIndex, err := strconv.Atoi(keyPart); err == nil {
				if !currentValue.cast(ValueSlice) {
					// keyPart is an integer, so it is expected to be either nil or slice.
					// malformed input, ignoring value
					continue keyLoop

				}
				currentValue = currentValue.to(ValueSlice)
				sliceIndex = sIndex
				continue
			}
			// currentValue is a slice, but if the code reaches here, the next value will be a map.
			// creates a slice element to host the new map element.
			if currentValue.Is(ValueSlice) {
				currentValue, err = currentValue.newNilValueAt(sliceIndex)
				if err != nil {
					// cannot set slice value for keyPart
					// malformed input, ignoring value
					continue keyLoop
				}
			}
			// creates a map element for keyPart
			currentValue, err = currentValue.mapFor(keyPart)
			if err != nil {
				// cannot set map key value for keyPart
				// malformed input, ignoring value
				continue keyLoop
			}
		}

		if currentValue.Is(ValueSlice) {
			// value is a slice, creates elements
			appendStrings(currentValue, sliceIndex, value...)
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

type Map interface {
	// Get navigates the map and tries to find the path to the value.
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
	Get(...any) (Value, error)
	// GetString() behaves the same as Get(), ignore errors and returns "" if the value is not a string
	// Note that GetString() will return "" also if the keys are correct but the string is an empty string.
	//  mapV.Get("a", "b") // returns "" if a.b is not a string or an empty string.
	GetString(keys ...any) string
	// Returns the form in an organized and clean format.
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
	// When callee on ValueNil and ValueString, it does nothing but returns ErrValueNotMapOrSlice
	// When descending the keys Value.Key() returns the key relative to the position in the map.
	Each(each func(Value) error) error
}

type valueWriter interface {
	setValue(any)
	to(ValueType) Value
	newNilValueAt(int) (Value, error)
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

type item struct {
	key       string
	value     any
	valueType ValueType
}

func (val *item) String() (value string, ok bool) {
	if val.value == nil {
		return "", true
	}
	value, ok = val.value.(string)
	return
}

func (val *item) setValue(v any) {
	val.value = v
}

func (val *item) Key() (value string) {
	return val.key
}
func (val *item) Type() (t ValueType) {
	return val.valueType
}

func (val *item) Slice() (value []Value, ok bool) {
	value, ok = (val.value).([]Value)
	return
}

func (val *item) IsNil() bool {
	return val.Is(ValueNil)
}

func (val *item) Map() (value map[string]Value, ok bool) {
	value, ok = (val.value).(map[string]Value)
	return
}

func (val *item) Is(t ValueType) bool {
	return val.valueType == t
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
			val.value = make([]Value, 0)
		}
	}
	if t == ValueMap {
		if !val.Is(ValueMap) {
			val.valueType = ValueMap
			val.value = make(map[string]Value, 0)
		}
	}
	return val
}

func (val *item) setString(k string) (Value, error) {
	if !val.cast(ValueMap) {
		return nil, ErrValueNotMap
	}
	m := (val.value).(map[string]Value)
	s, ok := m[k]
	if ok {
		if !s.(*item).cast(ValueString) {
			return nil, errKeyTypeMismatch(k, "string")
		}
	} else {
		m[k] = newStringValue(k)
	}
	return m[k], nil
}

func (val *item) mapFor(k string) (Value, error) {
	if !val.cast(ValueMap) {
		return nil, ErrValueNotMap
	}
	valueKey := ""
	if val.key == "" {
		valueKey = k
	} else {
		valueKey = fmt.Sprintf("%s[%s]", val.key, k)
	}
	m := (val.value).(map[string]Value)
	if s, ok := m[k]; ok {
		return s, nil
	} else {
		m[k] = newNilValue(valueKey)
	}
	return m[k], nil
}

func newStringValue(key string) Value {
	return &item{
		key:       key,
		value:     "",
		valueType: ValueString,
	}
}

func appendStrings(val Value, sliceIndex int, list ...string) error {
	if !val.Is(ValueSlice) {
		return ErrValueNotSlice
	}
	for i, str := range list {
		nested, _ := val.newNilValueAt(sliceIndex + i)
		nested.to(ValueString).setValue(str)
	}
	return nil
}

func (val *item) newNilValueAt(sliceIndex int) (Value, error) {
	if !val.Is(ValueSlice) {
		return nil, ErrValueNotSlice
	}
	slice, _ := (val.value).([]Value)
	if sliceIndex == -1 {
		sliceIndex = len(slice)
	}
	for i := 0; sliceIndex >= len(slice); i++ {
		k := fmt.Sprintf("%s[%d]", val.key, len(slice))
		slice = append(slice, newNilValue(k))
	}
	val.value = slice
	return slice[sliceIndex], nil
}

func (val *item) Get(keys ...any) (out Value, err error) {
	if len(keys) == 0 {
		return val, nil
	}
	out = val
	path := ""
	for kIndex, k := range keys {
		path += out.Key()
		if i, ok := k.(int); ok {
			if !out.Is(ValueSlice) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, value is not a slice", path, kIndex)
			}
			s, _ := out.Slice()
			if i >= len(s) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, out of range:%d", path, kIndex, i)
			}
			out = s[i]
		} else if s, ok := k.(string); ok {
			if !out.Is(ValueMap) {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, value is not a map", path, kIndex)
			}
			m, _ := out.Map()
			if out, ok = m[s]; !ok {
				return newNilValue(path), fmt.Errorf("%s invalid key at pos:%d, unknown key:%s", path, kIndex, s)
			}
		}
	}
	return
}

func (val *item) GetString(keys ...any) string {
	out, _ := val.Get(keys...)
	s, _ := out.String()
	return s
}

func (val *item) Each(each func(Value) error) error {
	if val.Is(ValueMap) {
		m, _ := val.Map()
		for _, v := range m {
			if err := each(v); err != nil {
				return err
			}
			v.Each(each)
		}
		return nil
	} else if val.Is(ValueSlice) {
		s, _ := val.Slice()
		for _, v := range s {
			if err := each(v); err != nil {
				return err
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

func newNilValue(key string) *item {
	return &item{
		key:       key,
		value:     "",
		valueType: ValueNil,
	}
}
