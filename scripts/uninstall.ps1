# Northstar — uninstall (PowerShell)

$ErrorActionPreference = "Stop"
$Root = if ($env:NORTHSTAR_HOME) { $env:NORTHSTAR_HOME } else { Join-Path $HOME ".northstar" }

if (-not (Test-Path $Root)) {
  Write-Host "Nothing to uninstall at $Root."
  exit 0
}

Write-Host "Northstar is installed at: $Root"
Write-Host ""

if (Test-Path "$Root\docker-compose.yml") {
  Write-Host "Stopping compose stack..."
  Push-Location $Root
  try { docker compose down | Out-Null } catch { }
  Pop-Location
}

$yn = Read-Host "Wipe data and config? [y/N]"
if ($yn -match '^(y|Y|yes|YES)$') {
  Write-Host "Removing $Root..."
  Remove-Item -Recurse -Force $Root
  Write-Host "Done. (Docker images are still cached — remove with: docker image prune)"
} else {
  Write-Host "Keeping $Root. Bring the stack back with:"
  Write-Host "    cd $Root; docker compose up -d"
}
