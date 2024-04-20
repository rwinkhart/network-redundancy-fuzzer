#!/bin/sh
gofmt -l -w -s ./*.go
git add -f .gitignore commit.sh go.mod go.sum LICENSE extra main.go README.md
git commit -m "$1"
git push
