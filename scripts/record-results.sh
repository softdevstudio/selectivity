#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
rm -rf test/results/latest
mkdir -p test/results/latest
{
 date -u '+%Y-%m-%dT%H:%M:%SZ'
 go version
 uname -sm
} > test/results/latest/environment.txt
go vet ./... > test/results/latest/vet.txt 2>&1
go test -race -count=1 -v ./... > test/results/latest/tests.txt 2>&1
sh scripts/demo.sh > test/results/latest/demo.txt 2>&1
cp demo-output/lines/summary.json test/results/latest/lines-summary.json
cp demo-output/tree/summary.json test/results/latest/tree-summary.json
printf 'All checks passed.\n'
