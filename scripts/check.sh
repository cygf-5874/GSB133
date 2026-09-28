#!/usr/bin/env bash
# 固定验收入口的薄封装：把参数原样转给仓库根的 check/（package main）。
#
#   bash scripts/check.sh                # 跑全部 8 个场景
#   bash scripts/check.sh -list          # 列出全部场景
#   bash scripts/check.sh --only cost    # 只跑一组
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

exec go run ./check "$@"