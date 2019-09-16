.PHONY: all get clean build push

GO ?= go

all: build

build: get
	${GO} build

get:
	dep ensure

clean:
	@rm -rf prometheus

push: clean
	cf push
