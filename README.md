<p align="right"><a href="./README.en.md">English</a></p>

# specbackfill / diff から companion obligation を抽出する deterministic change-contract compiler

specbackfill は、git diff から「この変更が発生させた **companion obligation**」を rule-based に抽出する **deterministic change-contract compiler** です。現行 v0 の `check` は、その obligation のうち同じ diff で companion evidence が見えないものを **diff-local omission** finding として報告します。

見るのは repo 全体の不足ではありません。**この diff が作った obligation と、それを満たす companion artifact が同じ diff で動いているか** です。

レビューで毎回出がちな、こういう追従漏れを diff だけから点検します。

- schema を変えたけど migration が同じ diff にない
- public API を変えたけど contract test が同じ diff にない
- env/config を増やしたけど default/docs が同じ diff にない
- authz 分岐を変えたけど allow/deny test が同じ diff にない
- worker/retry を変えたけど runbook/observability が同じ diff にない

現状の repository には、`specbackfill check` の v0 MVP、実装済みルールを確認する `specbackfill rules` コマンド、fixture coverage を見る `specbackfill fixtures` コマンド、検証 fixture が入っています。

この README は日本語の主入口です。挙動の正本は [docs/v0-spec.md](./docs/v0-spec.md)、変更時の制約は [AGENTS.md](./AGENTS.md) にあります。

## このリポジトリの中心

- [README.md](./README.md): 日本語の主入口
- [README.en.md](./README.en.md): 英語版
- [docs/v0-spec.md](./docs/v0-spec.md): v0 の source of truth
- [AGENTS.md](./AGENTS.md): 実装・文書変更時の制約
- [LICENSE](./LICENSE): ライセンス

## プロダクト境界

- 判定核は **rule-based** です。AI は後段で説明を整えても、finding 自体は発明しません。
- 中心概念は **companion obligation** です。finding は user-facing な未回収 obligation として扱います。
- finding は常に **diff-local** です。repo 全体の欠落や不存在は主張しません。
- すべての finding に **evidence** が必要です。証拠を示せない finding は出しません。
- v0 は diff 入力だけで成立します。PR タイトルや issue 文脈には依存しません。
- advisory-first です。pilot で有用性が確認されるまでは、blocking gate としての説明を前面に出しません。
- obligation/status JSON は `--emit-obligations` の versioned artifact として明示的に出します。通常の `--format json` は findings 契約のままです。
- `local-ai-review` に渡す場合は `--emit-local-ai-review-import` の JSONL を使えます。これは deterministic static layer の adapter であり、AI finding や PR comment は作りません。

`local-ai-review` のようなローカル LLM レビュー基盤と併用する場合、specbackfill は deterministic static layer として companion obligation output を出し、AI 側はそれを説明・整理・履歴化する役割に留めます。specbackfill 自体は AI finding を発明しません。

specbackfill は `local-ai-review` や `review-firewall` とは別物です。deterministic な rule ID、obligation semantics、evidence、fixture、CLI JSON の正本は specbackfill 側に置きます。`local-ai-review` は probabilistic review、history、prompt calibration を所有し、`review-firewall` は既存 review comment の triage/routing を所有します。

## v0 の契約

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json|markdown]
                  [--fail-on error|warn|off]
                  [--summary]
                  [--explain]
                  [--emit-obligations]
                  [--emit-local-ai-review-import]
```

実装済みルールを確認する場合は、次のコマンドを使います。

```bash
specbackfill rules list
specbackfill rules show DB001
```

この repository の開発中に実装済みルールの fixture coverage を確認する場合は、specbackfill repository root で次を実行します。

```bash
specbackfill fixtures report
```

- 入力: working tree diff / git range diff / unified diff file
- 出力: `text`、`json`、`markdown`
- 終了コード: `0` no findings, `1` findings at threshold, `2` tool error
- `--summary`: severity counts と fired rules だけを表示します。finding 判定は変えません。
- `--explain`: 既存 finding に紐づく grounded explanation を追加します。finding 自体は増やしません。
- `--emit-obligations`: `schema_version`, `tool`, `run`, `obligations` を持つ `obligations.v1` JSON artifact を出します。`satisfied` の companion evidence と `suppressed` の reason/evidence もこの artifact で確認できます。`--format` は省略するか `--format json` を指定します。
- `--emit-local-ai-review-import`: `local_ai_review_import.v1` JSONL を出します。各行は deterministic item ID、run ID、rule ID、status、severity、title、diff-local evidence digest、`source/import_kind` を持ちます。`--format` とは併用しません。
- JSON findings には deterministic な `finding_id` と `omission_signature` が入ります。
- 通常の `--format json` は findings 契約です。obligation/status artifact は [schemas/obligations.schema.json](./schemas/obligations.schema.json) に従う別契約です。
- local-ai-review import JSONL は [schemas/local_ai_review_import.schema.json](./schemas/local_ai_review_import.schema.json) に従う adapter 契約です。
- `rules`: 実装済み default v0 rule の ID、severity、意図、expected companions を表示します。diff は評価しません。
- `fixtures`: synthetic fixture coverage を rule ごとに表示します。diff は評価しません。

仕様の詳細と用語の正本は [docs/v0-spec.md](./docs/v0-spec.md) を見てください。

## インストール

利用者として試す場合は、まず `go install` で CLI を入れます。

```bash
go install github.com/mt4110/specbackfill/cmd/specbackfill@latest
specbackfill check --diff-file change.diff --format text --fail-on off
```

この repository からローカルに試す場合は、`make install` で `~/.local/bin/specbackfill` に入れられます。

```bash
make install
cd /path/to/another/project
specbackfill check --fail-on off
```

この repository 自体を開発する場合は、`make trial` や `go run ./cmd/specbackfill` でも確認できます。

## CI での利用

GitHub Actions では、PR の base/head をローカルに取得してから range diff として点検します。pilot thresholds が通るまでは `--fail-on off` で advisory output として流します。

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

      - name: Install specbackfill
        run: go install github.com/mt4110/specbackfill/cmd/specbackfill@latest

      - name: Run specbackfill
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
        run: |
          specbackfill check \
            --base "$BASE_SHA" \
            --head "$HEAD_SHA" \
            --format text \
            --fail-on off
```

- `fetch-depth: 0`: `--base/--head` の両方をローカルで参照できるようにします。
- `--format text`: CI log で人が読む用途に向きます。`--format json` は CI や downstream processing 向けです。
- `--fail-on off`: finding では CI を失敗させず、advisory output として確認できます。`warn` は `warn` と `error` で exit code `1`、`error` は `error` のみで exit code `1` にします。

## ローカル確認

`make test` は pure Go/Python の検証として動きます。mise を使う場合は任意で toolchain を揃えられます。

```bash
make install
make trial
make test
make check
make pr BASE=main HEAD=HEAD
make patch DIFF=testdata/patches/db001_positive.diff
make json
make md
make summary
make rules
make rule RULE=DB001
make fixtures
make pilot-eval
```

mise の task 定義も確認したい場合だけ、追加で `make test-mise` を使えます。

`make trial` はこの repository の self-check です。他の project を点検する場合は、まず `make install` で `specbackfill` コマンドを入れてから、対象 project の root で `specbackfill check --fail-on off` を実行します。

出力の `input:` 行は、どの差分を見たかを示します。`git range diff` の場合は未コミットの working tree 差分を含みません。`working tree diff` の場合、untracked files は `git add -N` などで git diff に見える状態にしない限り含みません。

`changed file summary:` は、評価された差分のファイルを docs、tests、migrations、API specs、Go source などにざっくり分けた読み取り補助です。各カテゴリには代表ファイルが最大 3 件だけ表示されます。検出結果や exit code は変えません。

## 試用チェックの完了目安

まずは `specbackfill check --fail-on off` または `make trial` 相当の advisory mode で実 diff に当てます。ざっくり次を満たせば、その diff の試用チェックは完了です。

- `make check` または `make pr BASE=main HEAD=HEAD` が tool error なく終わる
- finding が出た場合、rule ID、evidence、expected companions を見て判断できる
- 文言が repo-wide absence ではなく「この diff で obligation を満たす companion が動いていない」に留まっている
- うるさい・分かりにくい・外している finding は、次の synthetic fixture 候補として残す
- 品質改善に入る前に `make trial` または `make test` と `make fixtures` で現在地を確認する

繰り返し同じ種類のノイズが見えたら、 blocking に上げる前に fixture hardening と suppression を小さく入れます。

## Pilot scorecard

blocking へ上げる前に、実 diff の deterministic obligation output を scorecard で採点します。公開 repository には匿名・合成サンプルだけを置き、実 PR の title/body/comment、private review text、個人情報、raw private diff は保存しません。

```bash
specbackfill check --diff-file change.diff --emit-obligations --fail-on off > obligations.json
specbackfill check --diff-file change.diff --emit-local-ai-review-import --fail-on off > specbackfill-import.jsonl
python3 scripts/evaluate_pilot.py examples/pilot_scorecard.sample.csv --allow-small-sample --local-ai-review-import yes
```

scorecard 契約は [schemas/pilot_scorecard.schema.json](./schemas/pilot_scorecard.schema.json)、合成サンプルは [examples/pilot_scorecard.sample.csv](./examples/pilot_scorecard.sample.csv) にあります。判定は `continue`、`continue_advisory_only`、`archive` のいずれかです。`--allow-small-sample` はサンプル確認用で、`archive` 判定は実 pilot 相当の標本数に達した場合だけ有効になります。

`make pilot-eval` は合成サンプル確認用の既定値で動きます。実 pilot では `PILOT_SCORECARD=...` と `PILOT_EVAL_ARGS='--local-ai-review-import yes'` のように明示して使います。

pilot decision を公開 repository に残す場合は [examples/pilot_decision_record.template.md](./examples/pilot_decision_record.template.md) の項目を使い、`scripts/evaluate_pilot.py` の出力から匿名・集計値だけを記録します。実 pilot データがない場合は `Pilot status: not_run` / `Decision: pending` とし、数値を推測で埋めません。実 PR の title/body/comment、private review text、raw private diff、個人情報、proprietary repo 名は commit しません。合成サンプルは workflow 確認用であり、pilot decision の根拠にはしません。

実 pilot の public-safe aggregate がまだない場合は、評価済み decision record を作らず、template と README の運用だけを根拠に advisory のまま止めます。

## 実装済みルール

- `DB001`: Schema changed, no migration moved with the diff
- `DB002`: Destructive storage change, no rollback/backfill note
- `API001`: Public API surface changed, no contract test moved
- `CFG001`: New config/env/flag introduced, no docs/default moved
- `AUTH001`: Authn/Authz branch changed, no allow/deny tests or security-sensitive note moved
- `ERR001`: Public error/status/code contract changed, no assertion test moved
- `OPS001`: Worker/queue/retry behavior changed, no observability/runbook moved
- `DOC001`: Generated spec changed, no hand-written explanation moved

## Fixture coverage

specbackfill は positive、companion-present、negative、edge の synthetic diff fixtures で検証しています。この repository の開発中は、現在の coverage を次で確認できます。

```bash
specbackfill fixtures report
```

目的はルール数を増やすことではなく、evidence-backed な finding を静かに保つことです。

## 検出しない・主張しないこと

specbackfill は「repo に何かが存在しない」とは言いません。この diff で companion artifact が一緒に動いたかだけを見ます。

ノイズを避けるため、v0 は次のような diff では finding を抑える方針です。

- docs-only diffs
- tests-only diffs
- generated-file-only diffs
- example-only or top-level sample-only diffs where no production contract moved
- metadata-only renames
- companion artifacts that moved with concrete companion evidence

## 隣接ツールとの違い

specbackfill は general static analyzer、PR comment bot、team-policy script ではありません。

- Semgrep は code pattern や security/style issue を見つけます。
- Danger は team-specific な PR 雑務を自動化します。
- reviewdog は linter finding を diff 上に報告します。
- specbackfill は implementation change が作った companion obligation と、それを満たす expected companion artifacts が同じ diff で動いたかを点検します。

中心の finding は「このコードが間違っている」ではなく、「この diff は X という obligation を作ったが、companion Y が同じ diff で動いていない」です。

## ライセンス

[MIT License](./LICENSE)

## ステータス

- この repository は phase-limited に進めます。
- `docs/v0-spec.md` は v0 の source of truth です。
- README は入口に徹し、未実装の機能を実装済みのようには書きません。
