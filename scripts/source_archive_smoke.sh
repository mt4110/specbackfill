#!/usr/bin/env sh
set -eu

repo_root="${1:-.}"
repo_root=$(cd "$repo_root" && pwd)

resolve_go() {
	if command -v go >/dev/null 2>&1; then
		goroot=$(go env GOROOT 2>/dev/null || true)
		if [ -n "$goroot" ] && [ -x "$goroot/bin/go" ]; then
			printf '%s\n' "$goroot/bin/go"
			return
		fi
		command -v go
		return
	fi

	if command -v mise >/dev/null 2>&1; then
		goroot=$(mise exec -- go env GOROOT 2>/dev/null || true)
		if [ -n "$goroot" ] && [ -x "$goroot/bin/go" ]; then
			printf '%s\n' "$goroot/bin/go"
			return
		fi
		go_bin=$(mise exec -- which go 2>/dev/null || true)
		if [ -n "$go_bin" ] && [ -x "$go_bin" ]; then
			printf '%s\n' "$go_bin"
			return
		fi
	fi

	printf '%s\n' go
}

go_cmd=$(resolve_go)

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

archive="$tmpdir/source.tar"
copy="$tmpdir/source"

# Copy the checkout without local git/private state to approximate a source archive.
mkdir -p "$copy"
(cd "$repo_root" && tar \
	--exclude .git \
	--exclude .private_docs \
	--exclude .codex \
	--exclude '.codex-*' \
	--exclude docs/prompt \
	-cf "$archive" .)
(cd "$copy" && tar -xf "$archive")

cd "$copy"

echo '# source archive smoke'
echo "copy: $copy"

"$go_cmd" test ./...
python3 -m unittest scripts/evaluate_pilot_test.py
SPECBACKFILL_GO="$go_cmd" python3 scripts/schema_validate_testdata.py --repo-root .
