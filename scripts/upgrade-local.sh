#!/usr/bin/env bash
# Wait for a tag's Release workflow, confirm the GitHub release and Homebrew
# formula are published, then upgrade the local Homebrew install.
#
# Usage: scripts/upgrade-local.sh [TAG]   (default: latest vX.Y.Z tag)
# Brew steps are skipped when Homebrew is not installed.
set -euo pipefail

REPO="tackhq/tack"
TAP_REPO="tackhq/homebrew-tap"
TAP="tackhq/tap"
FORMULA="tack"

TAG="${1:-}"
if [ -z "$TAG" ]; then
  git fetch --tags -q origin 2>/dev/null || true
  TAG=$(git tag --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
fi
if [ -z "$TAG" ]; then
  echo "error: no release tag found" >&2
  exit 1
fi
VERSION="${TAG#v}"
echo "==> Release $TAG"

command -v gh >/dev/null || { echo "error: gh CLI is required" >&2; exit 1; }

# 1. Release workflow for the tag
RUN_ID=$(gh run list -R "$REPO" --workflow Release --branch "$TAG" -L 1 \
  --json databaseId -q '.[0].databaseId // empty')
if [ -z "$RUN_ID" ]; then
  echo "error: no Release workflow run found for $TAG (was the tag pushed?)" >&2
  exit 1
fi
STATUS=$(gh run view -R "$REPO" "$RUN_ID" --json status,conclusion -q '.status + " " + .conclusion')
if [ "${STATUS%% *}" != "completed" ]; then
  echo "==> Waiting for Release workflow (run $RUN_ID)..."
  gh run watch -R "$REPO" "$RUN_ID" --exit-status --interval 20 >/dev/null || {
    echo "error: Release workflow failed: $(gh run view -R "$REPO" "$RUN_ID" --json url -q .url)" >&2
    exit 1
  }
elif [ "${STATUS#* }" != "success" ]; then
  echo "error: Release workflow concluded '${STATUS#* }': $(gh run view -R "$REPO" "$RUN_ID" --json url -q .url)" >&2
  exit 1
fi
echo "==> Release workflow succeeded"

# 2. GitHub release published
gh release view -R "$REPO" "$TAG" --json url -q '"==> Published: " + .url' || {
  echo "error: GitHub release $TAG not found" >&2
  exit 1
}

# 3. Homebrew formula updated in the tap
TAP_VERSION=$(gh api "repos/$TAP_REPO/contents/Formula/$FORMULA.rb" -q .content \
  | base64 -d | sed -n 's/^ *version "\(.*\)"/\1/p' | head -1)
if [ "$TAP_VERSION" != "$VERSION" ]; then
  echo "error: tap formula is at $TAP_VERSION, expected $VERSION" >&2
  exit 1
fi
echo "==> Tap formula at $TAP_VERSION"

# 4. Local upgrade
if ! command -v brew >/dev/null; then
  echo "==> Homebrew not installed; skipping local upgrade"
  exit 0
fi

TAP_DIR=$(brew --repository "$TAP")
if [ -d "$TAP_DIR/.git" ]; then
  git -C "$TAP_DIR" pull --ff-only -q
else
  brew tap "$TAP"
fi

if brew list --formula "$TAP/$FORMULA" >/dev/null 2>&1; then
  HOMEBREW_NO_AUTO_UPDATE=1 brew upgrade "$TAP/$FORMULA" || true
else
  HOMEBREW_NO_AUTO_UPDATE=1 brew install "$TAP/$FORMULA"
fi

INSTALLED=$("$(brew --prefix)/bin/$FORMULA" --version 2>/dev/null | awk '{print $3}')
if [ "$INSTALLED" != "$VERSION" ]; then
  echo "error: installed $FORMULA is $INSTALLED, expected $VERSION" >&2
  exit 1
fi
echo "==> Local $FORMULA is $INSTALLED"
