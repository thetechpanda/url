# url

[![Go Report Card](https://goreportcard.com/badge/github.com/thetechpanda/url)](https://goreportcard.com/report/github.com/thetechpanda/url)
[![Go Reference](https://pkg.go.dev/badge/github.com/thetechpanda/url.svg)](https://pkg.go.dev/github.com/thetechpanda/url)
[![Release](https://img.shields.io/github/release/thetechpanda/url.svg?style=flat-square)](https://github.com/thetechpanda/url/tags/latest)

The `url` package simply parses `url.Values` and returns it as structured and organized map.

## Why?

Let's look at the payload below (added spaces to make it clearer to read)

```text
value=a & value=b & value=c
```

Does not require any parsing, `url.Values`'s `Get()` is more than sufficient to handle it.
However when the payload becomes more convoluted like the one below.

```text
slice2[1]=a & slice2[2]=b & map0[key]=c & map1[key][subKey]=d & map2[sub][1]=e & map2[sub][2]=f
```

Parsing it becomes complexer and most of the time it requires ad-hoc code to handle the specific structure of the data. Hence the package.

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
    v, err := valueMap.GetValue("key","sub")
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

Please refer to the comments in `main.go` or go to [package documentation](https://pkg.go.dev/github.com/thetechpanda/url)

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

## Testing and Benchmark

You can use `make test` or `make bench` to run the benchmarks.
Please note that in order to stress the tool `main_test.go` generates random data each time it runs.

Benchmark results in my lab

```text
make bench
mkdir -p bin/ prof/
go test -c -o bin/url.test
bin/url.test -test.cpu 1 -test.benchmem -test.run=./... -test.bench=./... -test.cpuprofile=prof/cpuprof -test.memprofile=prof/memprof
goos: linux
goarch: amd64
pkg: github.com/thetechpanda/url
cpu: Intel(R) Xeon(R) CPU E5-2678 v3 @ 2.50GHz
BenchmarkParse/url.Values_count/1               305806          7160 ns/op         2664 B/op          41 allocs/op
BenchmarkParse/url.Values_count/10               12826         94128 ns/op        27455 B/op         460 allocs/op
BenchmarkParse/url.Values_count/100               2023        987757 ns/op       281261 B/op        4449 allocs/op
BenchmarkParse/url.Values_count/1000               126      10016646 ns/op      2672150 B/op       40996 allocs/op
BenchmarkParse/url.Values_count/10000               16      97722209 ns/op     25745560 B/op      399359 allocs/op
BenchmarkParse/url.Values_count/100000               1    1228338786 ns/op    258457944 B/op     4036187 allocs/op
PASS
```

## Examples

Please refer to `main_test.go`

## Contributing

Fork, hack and submit the pull request :)

* Found a bug? Issue tracker is the way to go.
* Suggestion on how to optimize the code? Yes please!
