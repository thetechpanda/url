bench:
	mkdir -p bin/ prof/
	go test -c -o bin/url.test
	bin/url.test -test.cpu 1 -test.benchmem -test.run=./... -test.bench=./... -test.cpuprofile=prof/cpuprof -test.memprofile=prof/memprof
test:
	mkdir -p bin/ prof/
	go test -c -o bin/url.test
	bin/url.test -test.run=./... -test.cpuprofile=prof/cpuprof -test.memprofile=prof/memprof
pprof: test
	go tool pprof -http localhost:8081 bin/url.test prof/cpuprof