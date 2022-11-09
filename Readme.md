# url

The `url` package simply parses `url.Values` and returns it as structured and organize map.

`http.Request.Query()` and `http.Request.Form()`, return the keys without any aggregation, resulting in the raw key value passed through the PostBody or QueryString.

```text
value=a&
value=b&
value=c
```

Are straight forward and can be accessed by simply using `url.Values`'s `Get()` method, however:

```text
slice2[1]=a
slice2[2]=b
map0[key]=c
map1[key][subKey]=d
map2[sub][1]=e
map2[sub][2]=f
```

Are complexer to parse and most of the time they require ad-hoc code to handle the specific structure of the data.

## Install

```bash
go get github.com/thetechpanda/url
```

## Usage

```golang
package webStuff

import (
    URL "github.com/thetechpanda/url"
)

func dummyHandler(w http.ResponseWriter, r *http.Request) {

    valueMap, err := URL.ParseValues(r.PostForm())
    if err != nil {
        panic(err)
    }

    // verbose
    v, err := valueMap.Get("key","sub")
    if err != nil {
        panic(err)
    }

    s, ok := v.String()
    if !ok {
        panic("not a string")
    }

    // concise
    s := valueMap.GetString("key","sub")
    if s == "" {
        panic("invalid key or empty value")
    }

    // do something
}
```

## Documentation

Please refer to the comments in `main.go` :)

### ParseValues()

Parse processes url.Values and returns a Map interface

This function does its best to aggregate and organize the data parsing url.Values
For example the input below:

```go
    "input4[key0] = value4[key0]"
    "input4[key1][subKey1] = value4[key1][subKey1]"
    "input4[key1][subKey2] = value4[key1][subKey2]"
    "input4[key2][0] = value4[key2][0]"
    "input4[key2][1] = value4[key2][1]"
```

Will be interpreted as

```go
    { "input4" :
      { "key0": "value4[key0]",
        "key1": { "subKey1": "value4[key1][subKey1]",
                  "subKey2": "value4[key1][subKey2]" }
        "key2": [ "value4[key2][0]", "value4[key2][1]"]
      }
    }
```

#### Caveats

When cycling url.Values with range keys should be returned in the order submit by the browser/client,
however http.Request.Form() and http.Request.URL.Query() have different key order for identical request payloads.
This behavior makes it hard to handle array values, especially when used as root for hash values.
To provide output predictability Parse() sorts the keys by name when processing the url.Values using

```go
    sort.Strings(keys)
```

* When hash values after an array value, array key index should be defined.

```go
    "input[][key1] = a"
    "input[][key2] = b"
```

expected but not guaranteed result

```go
    "{ input : [ 0 => { key1 : a }, 1 => { key2 : b } ] }"
```

* The behavior when array values have mixed "[]" and "[%d]" format is not guaranteed report all values.

```go
    "input[0] = a"
    "input[1] = b"
    "input[] = c"
    "input[] = d"
```

expected but not guaranteed result

```go
    "{ input : [ 0 => a, 1 => b, 2 => c, 3 => e ] } "
```

* ValueNil typed Values could be in a ValueSlice typed Value if the indexes were missing or as a result of the previous comment.

```go
    "input[3] = a"
    "input[1] = b"
    "input[2] = c"
    "input[5] = d"
```

result would be

```go
    "{ input : [ 0 => , 1 => b, 2 => c, 3 => a, 4 => , 5 => d ] }"
```

* When an element has been identified as a Map or Slice, subsequent Keys must respect the type.

```go
    "input[0] = a"
    "input[key] = b"
    "input[2] = c"
```

depending on which value is parsed first, could be

```go
        "{ input : [ 0 => , 2 => c ] }"
     // or
        "{ input : { key: b } }"
```

* Malformed Key/Value pairs are ignored

## Examples

Please refer to `main_test.go`

## Contributing

Fork, hack and submit the pull request :)

Found a bug? Issue tracker is the place to go.
