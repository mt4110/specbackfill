#!/usr/bin/env sh
set -eu

repo_root="${1:-.}"
repo_root=$(cd "$repo_root" && pwd)

resolve_go() {
	if [ -n "${SPECBACKFILL_GO:-}" ]; then
		printf '%s\n' "$SPECBACKFILL_GO"
		return
	fi

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
version="${VERSION:-v0.0.0-smoke}"
commit="${COMMIT:-$(git -C "$repo_root" rev-parse --short HEAD 2>/dev/null || printf unknown)}"
built="${BUILT:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"

reject_whitespace() {
	name="$1"
	value="$2"
	case "$value" in
		*" "*|*"	"*)
			echo "error: $name must not contain whitespace" >&2
			exit 2
			;;
	esac
}

reject_whitespace VERSION "$version"
reject_whitespace COMMIT "$commit"
reject_whitespace BUILT "$built"

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

binary="$tmpdir/specbackfill"

cd "$repo_root"

echo '# release smoke'
"$go_cmd" build -ldflags "-X main.version=$version -X main.commit=$commit -X main.built=$built" -o "$binary" ./cmd/specbackfill

"$binary" --version | grep -F "specbackfill $version commit=$commit built=$built" >/dev/null
"$binary" rules list >/dev/null
"$binary" check --diff-file testdata/patches/db001_positive.diff --fail-on off >/dev/null

"$binary" check --diff-file testdata/patches/db001_positive.diff --format json --fail-on off >"$tmpdir/findings.json"
grep -F '"version": "v0"' "$tmpdir/findings.json" >/dev/null

"$binary" check --diff-file testdata/patches/db001_positive.diff --emit-obligations --fail-on off >"$tmpdir/obligations.json"
grep -F "\"version\": \"$version\"" "$tmpdir/obligations.json" >/dev/null

"$binary" check --diff-file testdata/patches/db001_positive.diff --emit-local-ai-review-import --fail-on off >"$tmpdir/specbackfill-import.jsonl"
grep -F "\"tool_version\":\"$version\"" "$tmpdir/specbackfill-import.jsonl" >/dev/null

echo "OK: release smoke passed for $version"
