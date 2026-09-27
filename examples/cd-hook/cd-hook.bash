# find-uncommitted — bash/zsh cd hook (Fork A ambient pre-flight)
#
# Source from ~/.bashrc or ~/.zshrc:
#   source /path/to/find-uncommitted/examples/cd-hook/cd-hook.bash
#
# Env:
#   FIND_UNCOMMITTED_BIN   Path to the binary (default: find-uncommitted on PATH)
#   FIND_UNCOMMITTED_CD_ARGS  Extra args before "check" (e.g. --no-remote)
#   FIND_UNCOMMITTED_CD_HOOK=0  Disable the hook without unsourcing

# Resolve binary once at source time; re-check exists on each invoke.
_find_uncommitted_bin() {
	if [ -n "${FIND_UNCOMMITTED_BIN:-}" ]; then
		printf '%s' "$FIND_UNCOMMITTED_BIN"
		return
	fi
	command -v find-uncommitted 2>/dev/null || true
}

# Run check for $PWD. Print only when Attention (exit 2). Never block cd.
_find_uncommitted_cd_check() {
	[ "${FIND_UNCOMMITTED_CD_HOOK:-1}" = "0" ] && return 0

	local bin
	bin="$(_find_uncommitted_bin)"
	[ -n "$bin" ] && [ -x "$bin" ] || return 0

	# Skip non-directories (shouldn't happen after a successful cd).
	[ -d "$PWD" ] || return 0

	# Fast path: not inside a git work tree → stay silent (no check spawn).
	# Mirror check's resolveGitToplevel: only when git says we have a toplevel.
	if ! git -C "$PWD" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
		_FIND_UNCOMMITTED_LAST_TOP=""
		return 0
	fi

	# Avoid re-check when still in the same work tree (subdir cds).
	# Leaving the tree clears the marker so re-entry runs check again.
	local top
	top="$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null)" || {
		_FIND_UNCOMMITTED_LAST_TOP=""
		return 0
	}
	if [ -z "$top" ]; then
		_FIND_UNCOMMITTED_LAST_TOP=""
		return 0
	fi
	if [ "${_FIND_UNCOMMITTED_LAST_TOP:-}" = "$top" ]; then
		return 0
	fi
	_FIND_UNCOMMITTED_LAST_TOP="$top"

	local out
	local ec=0
	local -a extra=()
	if [ -n "${FIND_UNCOMMITTED_CD_ARGS:-}" ]; then
		# Word-split user-provided extra args (e.g. "--no-remote").
		# shellcheck disable=SC2206
		extra=(${FIND_UNCOMMITTED_CD_ARGS})
	fi
	out="$("$bin" "${extra[@]}" check "$PWD" 2>/dev/null)" || ec=$?

	# 2 = Attention — the ambient moment. 0 = clear; 1 = error / not a repo.
	if [ "$ec" -eq 2 ] && [ -n "$out" ]; then
		printf '%s\n' "$out"
	fi
	return 0
}

# --- bash: wrap builtin cd ---
if [ -n "${BASH_VERSION:-}" ]; then
	cd() {
		builtin cd "$@" || return
		_find_uncommitted_cd_check
	}
fi

# --- zsh: chpwd hook (runs after any directory change) ---
if [ -n "${ZSH_VERSION:-}" ]; then
	autoload -Uz add-zsh-hook 2>/dev/null || true
	if typeset -f add-zsh-hook >/dev/null 2>&1; then
		add-zsh-hook chpwd _find_uncommitted_cd_check
	else
		# Fallback when add-zsh-hook is unavailable.
		chpwd_functions+=(_find_uncommitted_cd_check)
	fi
fi
