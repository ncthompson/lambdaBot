export GOPATH := $(shell pwd)
export PATH := $(PATH):$(shell pwd)/bin
ifeq ("$(origin COMPILER)", "command line")
COMPILER = $(COMPILER)
endif
ifndef COMPILER
COMPILER = gc
endif

ifndef GOPATH
$(error GOPATH is not defined)
endif

clean:
	rm -rf bin pkg build

gofmt:
	gofmt -l -s -w src/lambdie

install:
	@cd src && go install -v lambdie/...


.PHONY: init gofmt clean zip install

.DEFAULT_GOAL:=install