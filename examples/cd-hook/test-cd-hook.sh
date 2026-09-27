#!/usr/bin/env bash
# Smoke-test examples/cd-hook/cd-hook.bash against a built binary.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
BIN="${FIND_UNCOMMITTED_BIN:-$ROOT/binaries/find-uncommitted}"
HOOK="$ROOT/examples/cd-hook/cd-hook.bash"

if [ ! -x "$BIN" ]; then
	echo "Building $BIN ..."
	mkdir -p "$ROOT/binaries"
	(cd "$ROOT" && go build -o "$BIN" .)
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Dirty repo → check exits 2 → hook should print.
DIRTY="$TMP/dirty"
mkdir -p "$DIRTY"
git -C "$DIRTY" init -q
git -C "$DIRTY" config user.email "test@example.com"
git -C "$DIRTY" config user.name "Test"
echo hi >"$DIRTY/file.txt"
# untracked → local Attention

# Empty repo (no commits) → check exits 0 → hook should stay silent.
# (A committed repo without upstream is Attention: untracked-upstream.)
CLEAN="$TMP/clean"
mkdir -p "$CLEAN"
git -C "$CLEAN" init -q

# Non-git dir → silent, no error.
NONGIT="$TMP/nongit"
mkdir -p "$NONGIT"

export FIND_UNCOMMITTED_BIN="$BIN"
export FIND_UNCOMMITTED_CD_ARGS=--no-remote
export FIND_UNCOMMITTED_CD_HOOK=1
unset _FIND_UNCOMMITTED_LAST_TOP || true

# Fresh bash so the sourced cd wrapper is isolated.
run_case() {
	local label="$1" target="$2" expect_output="$3" # expect_output: yes|no
	local out
	out="$(
		bash -c '
			set -e
			source "$1"
			# Reset same-tree cache between cases when invoked in one shell;
			# each bash -c is fresh anyway.
			cd "$2"
		' bash "$HOOK" "$target" 2>&1
	)" || true

	if [ "$expect_output" = yes ]; then
		if [ -z "$out" ]; then
			echo "FAIL: $label — expected Attention output, got silence"
			echo "  target=$target"
			exit 1
		fi
		echo "OK:   $label — printed Attention ($(echo "$out" | wc -l) lines)"
	else
		if [ -n "$out" ]; then
			echo "FAIL: $label — expected silence, got:"
			printf '%s\n' "$out"
			exit 1
		fi
		echo "OK:   $label — silent"
	fi
}

echo "Binary: $BIN"
run_case "non-git directory" "$NONGIT" no
run_case "clean git repo" "$CLEAN" no
run_case "dirty git repo" "$DIRTY" yes

# Same work tree: second cd into subdir should not re-print.
SUB="$DIRTY/sub"
mkdir -p "$SUB"
same_tree_out="$(
	bash -c '
		source "$1"
		cd "$2"
		echo "---SEP---"
		cd "$3"
	' bash "$HOOK" "$DIRTY" "$SUB" 2>&1
)" || true
first_part="${same_tree_out%%---SEP---*}"
second_part="${same_tree_out#*---SEP---}"
first_part="$(printf '%s' "$first_part" | sed '/^$/d')"
second_part="$(printf '%s' "$second_part" | sed '/^$/d')"
if [ -z "$first_part" ]; then
	echo "FAIL: dirty repo first cd produced no output"
	exit 1
fi
if [ -n "$second_part" ]; then
	echo "FAIL: same-tree subdir cd re-printed:"
	printf '%s\n' "$second_part"
	exit 1
fi
echo "OK:   same work tree — second cd silent"

# Leave work tree then re-enter — cache must clear so Attention prints again.
reenter_out="$(
	bash -c '
		source "$1"
		cd "$2" >/dev/null
		cd "$3" >/dev/null
		cd "$2"
	' bash "$HOOK" "$DIRTY" "$NONGIT" 2>&1
)" || true
if [ -z "$reenter_out" ]; then
	echo "FAIL: re-enter after leave — expected Attention output"
	exit 1
fi
echo "OK:   leave then re-enter — printed Attention"

# Disable flag
out="$(
	bash -c '
		export FIND_UNCOMMITTED_CD_HOOK=0
		source "$1"
		cd "$2"
	' bash "$HOOK" "$DIRTY" 2>&1
)" || true
if [ -n "$out" ]; then
	echo "FAIL: FIND_UNCOMMITTED_CD_HOOK=0 should silence"
	exit 1
fi
echo "OK:   FIND_UNCOMMITTED_CD_HOOK=0 — silent"

echo "All cd-hook bash smoke tests passed."
