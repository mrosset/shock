include $(GOROOT)/src/Make.inc

TARG=shock
GOFILES=shock.go twitter.go git.go shell.go
GOFMT=gofmt -l -w

include $(GOROOT)/src/Make.cmd

test: all
	./shock -s

format:
	${GOFMT} .
