#!/usr/bin/env bash
set -euo pipefail

UPSTREAM_REPO="https://github.com/nianhua99/PandoraHelper.git"
UPSTREAM_SHA="f6275d3dc4135a98a4e9a3957eee554d64cb4e25"
LAST_FUNCTIONAL_SHA="1186fa9869d06e79a491da06e7bf320aa5cad24d"

workdir="$(mktemp -d)"
trap 'rm -rf "$workdir"' EXIT

printf 'Importing PandoraHelper baseline %s\n' "$UPSTREAM_SHA"

git -C "$workdir" init -q
git -C "$workdir" remote add origin "$UPSTREAM_REPO"
git -C "$workdir" fetch --depth 1 origin "$UPSTREAM_SHA"
git -C "$workdir" checkout --detach -q FETCH_HEAD

# Keep GPT Mirror project-control files while importing the upstream application tree.
# In particular, do not replace our README or root GitHub Actions workflows.
rsync -a \
  --exclude='/.git/' \
  --exclude='/README.md' \
  --exclude='/.github/workflows/' \
  "$workdir/" ./

cat > UPSTREAM_BASELINE.md <<EOF
# PandoraHelper baseline

- Upstream repository: https://github.com/nianhua99/PandoraHelper
- Imported repository state: \`$UPSTREAM_SHA\`
- Imported repository-state date: 2025-07-01
- Last observed functional (non-README-only) commit before that state: \`$LAST_FUNCTIONAL_SHA\` (2024-12-25)

## Import policy

The baseline is intentionally imported before protocol modernization or dependency upgrades so later regressions can be attributed to a specific change.

The following GPT Mirror project-control files are preserved instead of being overwritten by upstream:

- root \`README.md\`
- root \`.github/workflows/\`
- GPT Mirror roadmap/architecture documents already present in this repository

All other imported application/source files retain their upstream paths at this stage. Legacy integrations may be broken; they are documented and refactored in later milestones rather than silently fixed during the baseline import.
EOF

cat > THIRD_PARTY_NOTICES.md <<EOF
# Third-party notices

## PandoraHelper

GPT Mirror uses source code derived from **PandoraHelper** as its initial engineering baseline.

- Project: PandoraHelper
- Repository: https://github.com/nianhua99/PandoraHelper
- Baseline commit: \`$UPSTREAM_SHA\`
- License: MIT

The upstream MIT license and copyright notice are retained in the imported \`LICENSE\` file and must remain with substantial portions of derived source as required by that license.

PandoraHelper itself credits additional upstream projects and dependencies. Their applicable licenses remain in their respective files/dependency metadata and must continue to be respected.
EOF

printf 'Baseline files staged in working tree.\n'
