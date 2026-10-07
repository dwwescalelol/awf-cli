#!/bin/sh
set -e
cd "$(dirname "$0")/.."
targets=${*:-$(node -p 'process.platform + "-" + process.arch')}
for target in $targets; do
  sh scripts/bundle.sh "$target"
  npx vsce package --no-dependencies --target "$target" --allow-missing-repository --skip-license
done
