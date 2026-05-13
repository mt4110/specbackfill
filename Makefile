SHELL := /bin/sh

SPECBACKFILL := mise exec -- go run ./cmd/specbackfill
PREFIX ?= $(HOME)/.local
BINDIR ?= $(PREFIX)/bin
BASE ?= main
HEAD ?= HEAD
DIFF ?=
FORMAT ?= text
RULE ?= DB001

.DEFAULT_GOAL := help

.PHONY: help install trial test check pr patch summary json md rules rule fixtures

help:
	@printf '%s\n' \
		'Targets:' \
		'  make install                Install specbackfill to BINDIR, default ~/.local/bin' \
		'  make trial                  Run the readable local trial checklist' \
		'  make check                  Run advisory check on the working tree diff' \
		'  make pr [BASE=main HEAD=HEAD]  Run advisory check on a git range' \
		'  make patch DIFF=change.diff Run advisory check on a unified diff file' \
		'  make summary                Show summary for BASE/HEAD' \
		'  make json                   Show JSON report for BASE/HEAD' \
		'  make md                     Show Markdown report for BASE/HEAD' \
		'  make rules                  List implemented rules' \
		'  make rule [RULE=DB001]      Show one rule' \
		'  make fixtures               Show fixture coverage' \
		'  make test                   Run tests'

install:
	@mkdir -p "$(BINDIR)"
	@mise exec -- go build -o "$(BINDIR)/specbackfill" ./cmd/specbackfill
	@printf 'Installed: %s/specbackfill\n' "$(BINDIR)"
	@case ":$$PATH:" in \
		*:"$(BINDIR)":*) printf 'OK: %s is on PATH.\n' "$(BINDIR)" ;; \
		*) printf 'Add this to your shell PATH if needed: export PATH="%s:$$PATH"\n' "$(BINDIR)" ;; \
	esac
	@printf 'Try in another project: cd /path/to/project && specbackfill check --fail-on off\n'

trial:
	@printf '\nThis is a self-check for the specbackfill repository.\n'
	@printf 'To check another project, run: make install, then cd there and run specbackfill check --fail-on off.\n'
	@printf '\n== 1/4 tests ==\n'
	@mise run test
	@printf '\n== 2/4 advisory diff check ==\n'
	@$(SPECBACKFILL) check --fail-on off
	@printf '\nReading this:\n'
	@printf '  - The "input:" line tells you which diff source was actually checked.\n'
	@printf '  - In --base/--head mode, uncommitted working tree changes are intentionally not included.\n'
	@printf '  - "No findings emitted." means the current diff has no detected companion-artifact omissions.\n'
	@printf '  - The "anchor scan:" line says whether any implemented v0 rule entrance matched before suppression.\n'
	@printf '  - If findings appear, inspect rule ID, evidence, and expected companions before changing code.\n'
	@printf '\n== 3/4 fixture coverage ==\n'
	@$(SPECBACKFILL) fixtures report
	@printf '\nReading this:\n'
	@printf '  - These are fixture counts, not failures.\n'
	@printf '  - Low negative counts point to future false-positive hardening candidates.\n'
	@printf '\n== 4/4 diff whitespace check ==\n'
	@git diff --check
	@printf 'OK: no diff whitespace problems.\n'
	@printf '\nTrial complete.\n'
	@printf 'Verdict:\n'
	@printf '  - local self-check completed\n'
	@printf '  - use the findings line above as the advisory result for this repo diff\n'
	@printf '  - fixture counts are informational; DB001/DB002 negatives are still future hardening candidates\n'
	@printf '\nNext useful command:\n'
	@printf '  make install\n'
	@printf '  cd /path/to/another/project\n'
	@printf '  specbackfill check --fail-on off\n'

test:
	@mise run test

check:
	@$(SPECBACKFILL) check --fail-on off

pr:
	@$(SPECBACKFILL) check --base "$(BASE)" --head "$(HEAD)" --format "$(FORMAT)" --fail-on off

patch:
	@test -n "$(DIFF)" || { echo 'usage: make patch DIFF=change.diff'; exit 2; }
	@$(SPECBACKFILL) check --diff-file "$(DIFF)" --format "$(FORMAT)" --fail-on off

summary:
	@$(SPECBACKFILL) check --base "$(BASE)" --head "$(HEAD)" --summary --fail-on off

json:
	@$(SPECBACKFILL) check --base "$(BASE)" --head "$(HEAD)" --format json --fail-on off

md:
	@$(SPECBACKFILL) check --base "$(BASE)" --head "$(HEAD)" --format markdown --fail-on off

rules:
	@$(SPECBACKFILL) rules list

rule:
	@$(SPECBACKFILL) rules show "$(RULE)"

fixtures:
	@$(SPECBACKFILL) fixtures report
