#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
embedded_dir="${repo_root}/internal/acquire/embedded/site-packages/linux-arm64"
output="${1:-${repo_root}/datk}"
python_version="3.12.8"
acquire_version="3.22"

cleanup() {
  rm -rf "${embedded_dir}"
  mkdir -p "${embedded_dir}"
  : > "${embedded_dir}/.gitkeep"
  printf 'Placeholder so go:embed has an embeddable file when generated acquire dependencies are absent.\n' > "${embedded_dir}/placeholder.txt"
}
trap cleanup EXIT

rm -rf "${embedded_dir}"
mkdir -p "${embedded_dir}"

if command -v uv >/dev/null 2>&1; then
  uv python install "${python_version}"
  uv pip install \
    --python "${python_version}" \
    --no-compile \
    --target "${embedded_dir}" \
    "acquire==${acquire_version}"
elif python3 - <<'PY' && python3 -m pip --version >/dev/null 2>&1; then
import sys
raise SystemExit(0 if sys.version_info[:3] == (3, 12, 8) else 1)
PY
  python3 -m pip install \
    --disable-pip-version-check \
    --no-compile \
    --target "${embedded_dir}" \
    "acquire==${acquire_version}"
else
  printf 'uv or python3.%s with pip is required to build embedded acquire artifacts\n' "${python_version}" >&2
  exit 1
fi

(cd "${repo_root}" && go build -o "${output}" .)
