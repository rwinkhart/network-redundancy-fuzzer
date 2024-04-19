#!/bin/sh
gofmt -l -w -s ./*.go
git add -f .gitignore commit.sh go.mod go.sum LICENSE extra main.go
git commit -m "$1"
git push
