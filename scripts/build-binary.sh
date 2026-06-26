#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output="${1:-${repo_root}/datk}"
python_version="3.12.8"
acquire_version="3.22"
target_goos="${GOOS:-$(go env GOOS)}"
target_goarch="${GOARCH:-$(go env GOARCH)}"
target_cgo_enabled="${CGO_ENABLED:-0}"
target_platform="${target_goos}-${target_goarch}"
embedded_root="${repo_root}/internal/acquire/embedded/site-packages"
embedded_dir="${embedded_root}/${target_platform}"

case "${target_platform}" in
  linux-arm64)
    python_platform="aarch64-unknown-linux-gnu"
    ;;
  linux-amd64)
    python_platform="x86_64-unknown-linux-gnu"
    ;;
  windows-amd64)
    python_platform="x86_64-pc-windows-msvc"
    ;;
  *)
    printf 'unsupported target platform %s; supported: linux-arm64, linux-amd64, windows-amd64\n' "${target_platform}" >&2
    exit 1
    ;;
esac

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
    --python-platform "${python_platform}" \
    --only-binary :all: \
    --no-compile \
    --target "${embedded_dir}" \
    "acquire==${acquire_version}"
elif [ "${target_goos}" = "$(go env GOOS)" ] && [ "${target_goarch}" = "$(go env GOARCH)" ] && python3 - <<'PY' && python3 -m pip --version >/dev/null 2>&1; then
import sys
raise SystemExit(0 if sys.version_info[:3] == (3, 12, 8) else 1)
PY
  python3 -m pip install \
    --disable-pip-version-check \
    --no-compile \
    --target "${embedded_dir}" \
    "acquire==${acquire_version}"
else
  printf 'uv is required for cross-platform embedded acquire artifacts; native builds may use python3.%s with pip\n' "${python_version}" >&2
  exit 1
fi

(cd "${repo_root}" && CGO_ENABLED="${target_cgo_enabled}" GOOS="${target_goos}" GOARCH="${target_goarch}" go build -o "${output}" .)
