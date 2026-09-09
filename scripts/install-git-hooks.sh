#!/usr/bin/env bash

set -Eeuo pipefail

repository_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
git -C "$repository_root" config core.hooksPath .githooks
printf 'Configured Git hooks from %s/.githooks.\n' "$repository_root"
