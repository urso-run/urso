#!/bin/sh
set -e
rm -rf completions
mkdir -p completions
for sh in bash zsh fish; do
	go run ./cmd/urso completion "$sh" >"completions/urso.$sh"
done
