#!/bin/sh
# Builds the TEST_SPEC.md traceability matrix from Allure result files.
#
#   spec-matrix.sh RESULTS_DIR            print the matrix
#   spec-matrix.sh RESULTS_DIR --write F  replace the matrix block in F
#   spec-matrix.sh RESULTS_DIR --check F  fail when the block in F is stale
set -eu

results=$1
mode=${2:-}
doc=${3:-}
start='<!-- matrix:start -->'
end='<!-- matrix:end -->'

matrix() {
	ls "$results"/*-result.json >/dev/null 2>&1 || { echo "no Allure results in $results" >&2; exit 1; }
	jq -rs '
		def lbl($n): ([.labels[]? | select(.name == $n) | .value] | first) // "";
		def cell: tostring | gsub("\\|"; "\\|");
		def layerOrder: {"Domain": 0, "Business logic": 1, "Data access": 2, "End to end": 3}[.] // 9;
		map(select((.testCaseId // "") | test("^([A-Z0-9]+-)+[0-9]{2}$")))
		| group_by(.testCaseId) | map(.[0])
		| sort_by((lbl("epic") | layerOrder), lbl("feature"), lbl("story"), .testCaseId)
		| (
			"| ID | Layer | Component.Method | Title | Style | Technique | Severity |",
			"| --- | --- | --- | --- | --- | --- | --- |",
			(.[] | "| `\(.testCaseId)` | \(lbl("epic")) | `\(lbl("feature")).\(lbl("story"))` | \(.name | sub("^\\[[^]]+\\] "; "") | cell) | \(lbl("tag")) | \(lbl("testTechnique")) | \(lbl("severity")) |"),
			"",
			"Total: \(length) cases."
		)
	' "$results"/*-result.json
}

block() {
	printf '%s\n\n' "$start"
	matrix
	printf '\n%s\n' "$end"
}

replace() {
	tmp=$(mktemp)
	block > "$tmp.block"
	awk -v start="$start" -v end="$end" -v blockfile="$tmp.block" '
		$0 == start { while ((getline line < blockfile) > 0) print line; skip = 1; next }
		$0 == end { skip = 0; next }
		!skip { print }
	' "$1" > "$tmp"
	rm -f "$tmp.block"
	echo "$tmp"
}

case $mode in
"") matrix ;;
--write)
	tmp=$(replace "$doc")
	mv "$tmp" "$doc"
	;;
--check)
	tmp=$(replace "$doc")
	if ! diff -u "$doc" "$tmp"; then
		rm -f "$tmp"
		echo "$doc: traceability matrix is stale; run make spec-matrix" >&2
		exit 1
	fi
	rm -f "$tmp"
	;;
*)
	echo "unknown mode: $mode" >&2
	exit 2
	;;
esac
