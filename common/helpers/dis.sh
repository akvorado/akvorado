#!/bin/bash
# Disassemble a symbol of this package as Go assembly. Jump targets become
# labels and the source location becomes a comment.
#
# Usage:
#   ./dis.sh <symbol-regexp>
#
# The package is built as a test binary, so tests and benchmarks are included.
# An inlined function keeps no symbol: disassemble a caller and read the
# file:line column to find it.
#
#   ./dis.sh 'BenchmarkAddrTo6\.func[123]$'

set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <symbol-regexp>" >&2
	exit 1
fi

bin=$(mktemp)
trap 'rm -f "$bin"' EXIT

go test -c -o "$bin" .

out=$(go tool objdump -s "$1" "$bin" | grep -v FUNCDATA) || true
if [ -z "$out" ]; then
	echo "$0: no symbol matches $1" >&2
	exit 1
fi

printf '%s\n' "$out" | awk '
function flush(   i, x, a, src, instr) {
	if (n == 0) return
	for (i = 1; i <= n; i++)
		if (addr[i] in tgt && !(addr[i] in lbl)) lbl[addr[i]] = "L" ++c
	for (i = 1; i <= n; i++) {
		$0 = line[i]; src = $1; a = $2
		$1 = $2 = $3 = ""; sub(/^[ \t]+/, ""); instr = $0
		for (x in lbl) gsub(x, lbl[x], instr)
		if (a in lbl) printf "%s:\n", lbl[a]
		printf "\t%-44s // %s\n", instr, src
	}
	n = 0
}
/^TEXT/ {
	name = $2; file = $3
	flush()
	delete line; delete addr; delete tgt; delete lbl; c = 0
	printf "\nTEXT %s // %s\n", name, file
	next
}
{
	n++; line[n] = $0; addr[n] = $2
	if ($4 ~ /^J/ && $5 ~ /^0x/) tgt[$5] = 1
}
END { flush() }'
