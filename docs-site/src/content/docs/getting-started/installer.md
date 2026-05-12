---
title: One-shot installer
description: A single-command install for Linux / macOS hosts.
---

If you'd rather skip the manual clone + compose dance, the project ships a
one-shot installer that puts everything in place and prints the pairing code at
the end.

## Linux / macOS

```sh
curl -fsSL https://raw.githubusercontent.com/brayden/northstar/main/scripts/install.sh | bash
```

What it does:

1. Verifies Docker + Docker Compose are installed.
2. Creates `~/.northstar/` and clones the compose + env templates into it.
3. Generates a unique server master passphrase (used to encrypt API keys).
4. Boots the stack in mock mode.
5. Mints a fresh pairing code and prints the QR-coded URL.

The script is short and auditable — read it first if that's your style:
[`scripts/install.sh`](https://github.com/brayden/northstar/blob/main/scripts/install.sh).

## Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/brayden/northstar/main/scripts/install.ps1 | iex
```

Same flow as the bash version, adapted for PowerShell + Docker Desktop.

## Reset / uninstall

```sh
bash ~/.northstar/uninstall.sh   # stops, prompts for data wipe, removes images
```

The installer never installs anything outside `~/.northstar/` and the Docker
images — there are no system services or kernel modules involved.
