#!/usr/bin/env bash
#
# Northstar — uninstall
#
# Stops the compose stack and optionally wipes the data volume + ~/.northstar/.

set -euo pipefail

ROOT="${NORTHSTAR_HOME:-$HOME/.northstar}"

if [[ ! -d "$ROOT" ]]; then
  echo "Nothing to uninstall at $ROOT."
  exit 0
fi

echo "Northstar is installed at: $ROOT"
echo

if [[ -f "$ROOT/docker-compose.yml" ]]; then
  echo "Stopping compose stack…"
  (cd "$ROOT" && docker compose down) || true
fi

read -r -p "Wipe data and config? [y/N] " yn
case "$yn" in
  y|Y|yes|YES)
    echo "Removing $ROOT…"
    rm -rf "$ROOT"
    echo "Done. (Docker images are still cached — remove with: docker image prune)"
    ;;
  *)
    echo "Keeping $ROOT. Bring the stack back with:"
    echo "    cd $ROOT && docker compose up -d"
    ;;
esac
