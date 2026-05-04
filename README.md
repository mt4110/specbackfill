<p align="right"><a href="./README.en.md">English</a></p>

# specbackfill / 実装差分の追従漏れを点検する CLI (diff-local omission CLI)

specbackfill は、コード差分から **companion artifacts** の追従漏れを **diff-local omission** として扱う、rule-based な CLI です。見るのは repo 全体の不足ではなく、**この diff で一緒に動くべきものが同じ diff で動いているか** です。

現状の repository には、`specbackfill check` の v0 MVP と実装済みルールの検証 fixture が入っています。

この README は日本語の主入口です。挙動の正本は [docs/v0-spec.md](./docs/v0-spec.md)、変更時の制約は [AGENTS.md](./AGENTS.md) にあります。

## このリポジトリの中心

- [README.md](./README.md): 日本語の主入口
- [README.en.md](./README.en.md): 英語版
- [docs/v0-spec.md](./docs/v0-spec.md): v0 の source of truth
- [AGENTS.md](./AGENTS.md): 実装・文書変更時の制約

## プロダクト境界

- 判定核は **rule-based** です。AI は後段で説明を整えても、finding 自体は発明しません。
- finding は常に **diff-local** です。repo 全体の欠落や不存在は主張しません。
- すべての finding に **evidence** が必要です。証拠を示せない finding は出しません。
- v0 は diff 入力だけで成立します。PR タイトルや issue 文脈には依存しません。
- AI レビューの前段で、小さな構造的ほつれを決定論的にすくうための CLI です。

`local-ai-review` のようなローカル LLM レビュー基盤と併用する場合、specbackfill は先に rule-based な omission finding を出し、AI 側はそれを説明・整理する役割に留めます。specbackfill 自体は AI finding を発明しません。

## v0 の契約

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json]
                  [--fail-on error|warn|off]
```

- 入力: working tree diff / git range diff / unified diff file
- 出力: `text` または `json`
- 終了コード: `0` no findings, `1` findings at threshold, `2` tool error

仕様の詳細と用語の正本は [docs/v0-spec.md](./docs/v0-spec.md) を見てください。

## CI での利用

GitHub Actions では、PR の base/head をローカルに取得してから range diff として点検します。

```yaml
name: specbackfill

on:
  pull_request:

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run specbackfill
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
        run: |
          go run ./cmd/specbackfill check \
            --base "$BASE_SHA" \
            --head "$HEAD_SHA" \
            --format text \
            --fail-on warn
```

- `fetch-depth: 0`: `--base/--head` の両方をローカルで参照できるようにします。
- `--format text`: CI log で人が読む用途に向きます。`--format json` は CI や downstream processing 向けです。
- `--fail-on warn`: `warn` と `error` で exit code `1` にします。`error` は `error` のみ、`off` は finding では失敗しません。

## ローカル確認

```bash
go test ./...
go run ./cmd/specbackfill check --diff-file testdata/patches/db001_positive.diff --format text --fail-on off
go run ./cmd/specbackfill check --diff-file testdata/patches/api001_err001_positive.diff --format json --fail-on off
```

## 実装済みルール

- `DB001`: Schema changed, no migration moved with the diff
- `DB002`: Destructive storage change, no rollback/backfill note
- `API001`: Public API surface changed, no contract test moved
- `CFG001`: New config/env/flag introduced, no docs/default moved
- `AUTH001`: Authn/Authz branch changed, no allow/deny tests or security-sensitive note moved
- `ERR001`: Public error/status/code contract changed, no assertion test moved
- `OPS001`: Worker/queue/retry behavior changed, no observability/runbook moved
- `DOC001`: Generated spec changed, no hand-written explanation moved

## ステータス

- この repository は phase-limited に進めます。
- `docs/v0-spec.md` は v0 の source of truth です。
- README は入口に徹し、未実装の機能を実装済みのようには書きません。
