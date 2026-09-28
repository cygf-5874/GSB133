#!/usr/bin/env bash
# 构建整个模块。
set -euo pipefail

cd "$(dirname "$0")/.."

exec go build ./...