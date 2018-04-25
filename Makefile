SHELL := /bin/bash

all: build

build: memconn.a
memconn.a: $(filter-out %_test.go, $(wildcard *.go))
	go build -o $@

GO_VERSION ?= 1.9.4
IMPORT_PATH := github.com/akutz/memconn

docker-run:
	docker run --rm -it \
      -v $$(pwd):/go/src/$(IMPORT_PATH) \
      golang:$(GO_VERSION) \
      make -C /go/src/$(IMPORT_PATH) $(MAKE_TARGET)

docker-strace:
	docker run --rm -it \
      --cap-add SYS_PTRACE \
      -v $$(pwd):/go/src/$(IMPORT_PATH) \
      strace \
      make -C /go/src/$(IMPORT_PATH) $(MAKE_TARGET)

BENCH ?= .

benchmark:
	go test -bench $(BENCH) -run Bench -benchmem .

benchmark-go1.9:
	MAKE_TARGET=benchmark $(MAKE) docker-run

test:
	go test
	go test -race -run 'Race$$'

test-go1.9:
	MAKE_TARGET=test $(MAKE) docker-run

strace-net-memconn:
	go test -c -o memconn.test || false
	@echo
	strace -f -e '!futex' ./memconn.test -test.v -test.run 'TestTLS_MemConn_NoTLS$$'
	@echo
	strace -f -e '!futex' ./memconn.test -test.v -test.run 'TestTLS_MemConn$$'

docker-strace-net-memconn:
	MAKE_TARGET=strace-net-memconn $(MAKE) docker-strace

strace-net-tcp:
	go test -c -o memconn.test || false
	@echo
	strace -f -e '!futex' ./memconn.test -test.v -test.run 'TestTLS_TCP$$'
	@echo
	strace -f -e '!futex' ./memconn.test -test.v -test.run 'TestTLS_TCP_NoTLS$$'
	
docker-strace-net-tcp:
	MAKE_TARGET=strace-net-tcp $(MAKE) docker-strace