#!/bin/bash
set -euo pipefail

project_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# The local CocoaPods installation is optional; a system pod takes precedence.
if ! command -v pod >/dev/null 2>&1 && [[ -x "$project_root/.tools/bin/pod" ]]; then
  export PATH="$project_root/.tools/bin:$PATH"
fi
if ! command -v pod >/dev/null 2>&1; then
  echo "CocoaPods is required. Install it before building the macOS app." >&2
  exit 1
fi

export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
cd "$project_root/trading_app"
exec flutter build macos "$@"
