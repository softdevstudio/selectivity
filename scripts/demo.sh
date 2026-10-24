#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
go build -o bin/selectivity ./cmd/selectivity
rm -rf demo-output
mkdir -p demo-output/source/nested
awk 'BEGIN { for(i=1;i<=20000;i++) {if(i==702) print "alpha"; else if(i==17891) print "omega"; else print "irrelevant-row-" i} }' > demo-output/large.txt
./bin/selectivity --input demo-output/large.txt --out demo-output/lines --repeats 1 -- sh -c 'grep -qx alpha "$1" && grep -qx omega "$1"' check '{input}'
printf 'alpha\nomega\n' > demo-output/expected
cmp demo-output/expected demo-output/lines/reduced
printf 'noise\nalpha\nextra\n' > demo-output/source/a.txt
printf 'noise\nomega\n' > demo-output/source/nested/b.txt
printf 'unneeded\n' > demo-output/source/junk.txt
./bin/selectivity --input demo-output/source --out demo-output/tree --lines-in-files --repeats 1 -- sh -c '[ -f "$1/a.txt" ] && [ -f "$1/nested/b.txt" ] && grep -qx alpha "$1/a.txt" && grep -qx omega "$1/nested/b.txt"' check '{input}'
printf 'alpha\n' | cmp - demo-output/tree/reduced/a.txt
printf 'omega\n' | cmp - demo-output/tree/reduced/nested/b.txt
test ! -e demo-output/tree/reduced/junk.txt
printf 'Both demonstrations passed.\n'
