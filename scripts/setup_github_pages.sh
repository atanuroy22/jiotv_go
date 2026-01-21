#!/usr/bin/env bash
set -euo pipefail
OWNER="${OWNER:-atanuroy22}"
REPO="${REPO:-jiotv_go}"
BRANCH="${BRANCH:-gh-pages}"
TOKEN_ENV="${TOKEN_ENV:-GITHUB_TOKEN}"
UNINSTALL="${UNINSTALL:-}"
TOKEN_VAR_INDIRECT="${TOKEN_ENV}"
TOKEN="${!TOKEN_VAR_INDIRECT:-}"
if [[ -z "${TOKEN}" ]]; then echo "Missing token in ${TOKEN_ENV}"; exit 1; fi
REMOTE="https://${TOKEN}@github.com/${OWNER}/${REPO}.git"
if [[ "${UNINSTALL}" == "1" || "${UNINSTALL}" == "true" ]]; then
  curl -s -H "Authorization: token ${TOKEN}" -H "Accept: application/vnd.github+json" -X DELETE "https://api.github.com/repos/${OWNER}/${REPO}/pages" || true
  git init
  git remote add origin "${REMOTE}" || true
  git push origin --delete "${BRANCH}" || true
  echo "Unlinked GitHub Pages"
  exit 0
fi
command -v git >/dev/null 2>&1 || { echo "git not found"; exit 1; }
BUILD_DIR="$(mktemp -d)"
if command -v mdbook >/dev/null 2>&1; then
  mdbook build docs
  cp -r docs/book/* "${BUILD_DIR}/"
else
  printf '%s\n' '<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>JioTV Go</title></head><body><h1>JioTV Go</h1><p>Documentation site</p><p><a href="./get_started.html">Get Started</a></p></body></html>' > "${BUILD_DIR}/index.html"
  cp -r docs/* "${BUILD_DIR}/"
fi
cp scripts/install.sh "${BUILD_DIR}/install.sh"
cp scripts/install.ps1 "${BUILD_DIR}/install.ps1"
touch "${BUILD_DIR}/.nojekyll"
(
  cd "${BUILD_DIR}"
  git init
  git checkout -b "${BRANCH}"
  git add -A
  git commit -m "Deploy Pages"
  git remote add origin "${REMOTE}"
  git push -f origin "${BRANCH}"
)
SITE_URL="https://${OWNER}.github.io/${REPO}/"
ok=0
for i in $(seq 1 30); do
  sleep 5
  if curl -s "${SITE_URL}" | grep -q "JioTV Go"; then ok=1; break; fi
done
if [[ "${ok}" -ne 1 ]]; then echo "Deployment verification failed"; exit 1; fi
echo "Deployment successful: ${SITE_URL}"
