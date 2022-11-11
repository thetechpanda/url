package url_test

import (
	"fmt"
	"math/rand"
	"net/url"
	"strings"
	"testing"
	"time"

	URL "github.com/thetechpanda/url"
)

var hashKeyCount = []int{1, 10, 100, 1000, 10000, 100000}
var err error

func generateRandomString(length int) string {
	var dictionary = []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z", "a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	rand.Shuffle(len(dictionary), func(i, j int) { dictionary[i], dictionary[j] = dictionary[j], dictionary[i] })
	if length > len(dictionary) {
		length = len(dictionary)
	}
	return strings.Join(dictionary[:length], "")
}

var cache = make(map[int]struct {
	d map[string][]any
	v url.Values
}, 0)

func generateTestData(qty int) (testData map[string][]any, urlValue url.Values) {
	if v, ok := cache[qty]; ok {
		return v.d, v.v
	}

	testData = make(map[string][]any)
	urlValue = make(url.Values)
	for i := 0; i < qty; i++ {
		root := generateRandomString(8)
		values := []any{root}
		keys := rand.Int() % 9
		for k := 0; k < keys; k++ {
			if rand.Int()%3 == 0 {
				root += fmt.Sprintf("[%d]", k)
				values = append(values, k)
			} else if rand.Int()%3 == 1 {
				root += "[]"
				values = append(values, 0)
			} else {
				key := generateRandomString(8)
				root += fmt.Sprintf("[%s]", key)
				values = append(values, key)
			}
		}

		testData[root] = values
		urlValue.Add(root, root)
	}
	cache[qty] = struct {
		d map[string][]any
		v url.Values
	}{
		testData, urlValue,
	}
	return
}

func TestParse(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	var hashCount = 100000
	var testData map[string][]any
	var urlValue url.Values
	var valueMap URL.Map
	t.Run(fmt.Sprintf("generating %d urlValues", hashCount), func(t *testing.T) {
		testData, urlValue = generateTestData(hashCount)
	})
	t.Run(fmt.Sprintf("parsing %d urlValues", hashCount), func(t *testing.T) {
		valueMap, err = URL.ParseValues(urlValue)
		if err != nil {
			t.Error(err)
			t.FailNow()
		}
	})

	t.Run(fmt.Sprintf("validating %d urlValues", hashCount), func(t *testing.T) {
		if err := validate(testData, valueMap); err != nil {
			t.Error(err)
		}
	})

	t.Run(fmt.Sprintf("testing %d KeyValue()", hashCount), func(t *testing.T) {
		valueMap.KeyValue()
	})
}

func validate(testData map[string][]any, valueMap URL.Map) error {
	for testKey, testArgs := range testData {
		v, err := valueMap.GetValue(testArgs...)
		if err != nil {
			return fmt.Errorf("failed %s: %v (%s)", testKey, testArgs, err.Error())
		}
		if !v.Is(URL.ValueString) {
			return fmt.Errorf("failed %s: %v (%s)", testKey, testArgs, "expected ValueString")
		}
		s := valueMap.GetString(testArgs...)
		if s != testKey {
			return fmt.Errorf("failed %s: %v (%s)", testKey, testArgs, "value mismatch")
		}
	}
	return nil
}

func benchmarkParse(testData map[string][]any, urlValue url.Values) (valueMap URL.Map, m map[string]string, err error) {

	valueMap, err = URL.ParseValues(urlValue)
	if err != nil {
		return nil, nil, err
	}

	if err := validate(testData, valueMap); err != nil {
		return nil, nil, err
	}
	return
}

var v URL.Map
var m map[string]string

func BenchmarkParse(b *testing.B) {
	for _, size := range hashKeyCount {
		generateTestData(size)
	}
	for _, size := range hashKeyCount {
		testData, urlValue := generateTestData(size)
		b.Run(fmt.Sprintf("url.Values_count/%d", size), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				if v, m, err = benchmarkParse(testData, urlValue); err != nil {
					panic(err)
				}
			}
		})
	}
}

func Example() {
	raw := make(url.Values)
	raw.Add("hello[]", "world")
	raw.Add("hello[]", "worlds")
	raw.Add("hello[]", "universe")
	raw.Add("map[key]", "map::value")
	mapV, err := URL.ParseValues(raw)
	if err != nil {
		panic(err)
	}

	fmt.Println("hello[] values ", mapV.GetStrings("hello"))
	fmt.Println("map[key] value ", mapV.GetString("map", "key"))
}
