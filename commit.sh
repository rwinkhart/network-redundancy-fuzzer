#!/bin/sh
gofmt -l -w -s ./*.go
git commit -am "$1"
git push
