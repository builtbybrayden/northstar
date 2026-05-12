# scripts/

One-shot installers and the corresponding uninstall scripts.

| Script | Platform |
|---|---|
| `install.sh` | Linux / macOS |
| `install.ps1` | Windows (PowerShell + Docker Desktop) |
| `uninstall.sh` | Linux / macOS |
| `uninstall.ps1` | Windows |

## Install

The README in the project root has the one-line `curl` / `iwr` invocations.
Both installers:

1. Verify Docker + Compose v2 are present.
2. Provision `~/.northstar/` with `docker-compose.yml`, `.env`, `litestream.yml`,
   plus the matching uninstall script.
3. Generate `NORTHSTAR_MASTER_PASSPHRASE` if one isn't already in `.env`.
4. Pull images and `docker compose up -d`.
5. Wait for `/api/health` to answer.
6. Mint a pairing code via `POST /api/pair/initiate` and print it.

Re-running an installer is idempotent — existing files are kept, only missing
ones are fetched. The data volume is never touched.

## Environment overrides

| Var | Default | Notes |
|---|---|---|
| `NORTHSTAR_REPO` | `brayden/northstar` | For forks |
| `NORTHSTAR_REF`  | `main` | Use a tag (`v1.0.0`) for reproducible installs |
| `NORTHSTAR_HOME` | `~/.northstar` | Install root |

Example: pin to a release tag

```sh
NORTHSTAR_REF=v1.0.0 \
  bash -c "$(curl -fsSL https://raw.githubusercontent.com/brayden/northstar/v1.0.0/scripts/install.sh)"
```

## Uninstall

```sh
bash ~/.northstar/uninstall.sh      # or .ps1
```

Prompts before wiping `~/.northstar/`. Never touches Docker images outside the
stack — leftover images can be pruned with `docker image prune`.
