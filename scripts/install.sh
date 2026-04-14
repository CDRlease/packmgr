#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${VERSION:-dev}"
GOPATH_DIR="$(go env GOPATH)"

if [[ -z "$GOPATH_DIR" ]]; then
  echo "go env GOPATH returned an empty value" >&2
  exit 1
fi

BIN_DIR="${GOPATH_DIR%/}/bin"
TARGET_BIN="${BIN_DIR}/packmgr"
SHELL_NAME="$(basename "${SHELL:-}")"

detect_rc_files() {
  case "$SHELL_NAME" in
    zsh)
      printf '%s\n' "$HOME/.zprofile"
      printf '%s\n' "$HOME/.zshrc"
      ;;
    bash)
      printf '%s\n' "$HOME/.bash_profile"
      printf '%s\n' "$HOME/.bashrc"
      ;;
    *)
      printf '%s\n' "$HOME/.profile"
      ;;
  esac
}

rewrite_path_block() {
  local rc_file="$1"
  local marker_start="# >>> packmgr GOPATH bin >>>"
  local marker_end="# <<< packmgr GOPATH bin <<<"
  local temp_file

  mkdir -p "$(dirname "$rc_file")"
  touch "$rc_file"

  temp_file="$(mktemp)"
  awk -v start="$marker_start" -v end="$marker_end" '
    $0 == start { skip = 1; next }
    $0 == end { skip = 0; next }
    !skip { print }
  ' "$rc_file" > "$temp_file"

  {
    cat "$temp_file"
    echo "$marker_start"
    echo 'case ":$PATH:" in'
    printf '  *":%s:"*) ;;\n' "$BIN_DIR"
    printf '  *) export PATH="%s:$PATH" ;;\n' "$BIN_DIR"
    echo 'esac'
    echo "$marker_end"
  } > "$rc_file"

  rm -f "$temp_file"
}

mkdir -p "$BIN_DIR"

echo "Building packmgr ${VERSION} into ${TARGET_BIN}"
go build -ldflags="-X main.Version=${VERSION}" -o "$TARGET_BIN" "$REPO_ROOT/cmd/packmgr"

UPDATED_FILES=()
while IFS= read -r rc_file; do
  rewrite_path_block "$rc_file"
  UPDATED_FILES+=("$rc_file")
done < <(detect_rc_files)

echo "Installed packmgr to ${TARGET_BIN}"
printf 'Updated PATH configuration in:%s\n' ""
for rc_file in "${UPDATED_FILES[@]}"; do
  printf '  %s\n' "$rc_file"
done
echo "Open a new shell, or source your shell config files."
