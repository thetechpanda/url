package url_test

import (
	"net/url"
	"testing"

	URL "github.com/thetechpanda/url"
)

var TestData = map[string][]any{
	"element1":          {"element1"},
	"slice1[]":          {"slice1", 0},
	"slice2[0]":         {"slice2", 0},
	"slice2[1]":         {"slice2", 1},
	"slice2[2]":         {"slice2", 2},
	"map0[key]":         {"map0", "key"},
	"map1[key][subKey]": {"map1", "key", "subKey"},
	"map2[sub][1]":      {"map2", "sub", 1},
	"map2[sub][2]":      {"map2", "sub", 2},
	"map2[sub][3]":      {"map2", "sub", 3},
	"map3[key][0][sub]": {"map3", "key", 0, "sub"},
	"map3[key][1][sub]": {"map3", "key", 1, "sub"},
	"map3[key][2][sub]": {"map3", "key", 2, "sub"},
	"slice3[0][sub][0]": {"slice3", 0, "sub", 0},
	"slice3[0][sub][1]": {"slice3", 0, "sub", 1},
	"slice3[0][sub][2]": {"slice3", 0, "sub", 2},
	"slice3[0][sub][3]": {"slice3", 0, "sub", 3},
}

func TestParse(t *testing.T) {
	urlValue := make(url.Values)
	for testKey := range TestData {
		urlValue.Add(testKey, testKey)
	}

	valueMap, err := URL.ParseValues(urlValue)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	for k, v := range valueMap.KeyValue() {
		t.Logf("%s = %s\n", k, v)
	}

	for testKey, testArgs := range TestData {
		v, err := valueMap.Get(testArgs...)
		if err != nil {
			t.Errorf("failed %s: %v (%s)", testKey, testArgs, err.Error())
			continue
		}
		if !v.Is(URL.ValueString) {
			t.Errorf("failed %s: %v (%s)", testKey, testArgs, "expected ValueString")
			continue
		}
		s := valueMap.GetString(testArgs...)
		if s != testKey {
			t.Errorf("failed %s: %v (%s)", testKey, testArgs, "value mismatch")
			continue
		}

	}
}
