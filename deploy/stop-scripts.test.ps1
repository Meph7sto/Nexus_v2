$ErrorActionPreference = "Stop"

function Assert {
    param([bool]$Condition, [string]$Message)

    if (-not $Condition) {
        throw $Message
    }
}

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$rootStart = Get-Content -LiteralPath (Join-Path $repoRoot "start.bat") -Raw
$rootStop = Get-Content -LiteralPath (Join-Path $repoRoot "stop.bat") -Raw
$deployStop = Get-Content -LiteralPath (Join-Path $PSScriptRoot "stop-windows.cmd") -Raw
$rootStartCommands = @($rootStart -split "\r?\n" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })

Assert ($rootStop -notmatch "(?im)^\s*pause\s*$") "stop.bat must not wait for manual terminal closure."
Assert ($deployStop -notmatch "(?im)^\s*pause\s*$") "deploy\stop-windows.cmd must not wait for manual terminal closure."
Assert ($rootStop -notmatch "(?i)/IM\s+(cmd|powershell)(\.exe)?") "stop.bat must not kill generic terminal processes."
Assert ($rootStartCommands[-1].Trim() -eq "exit /b 0") "start.bat must close automatically after a successful startup."

foreach ($title in @(
    "Nexus Docker App Logs",
    "Nexus Docker Postgres Logs",
    "Nexus Docker Redis Logs"
)) {
    Assert ($rootStop.Contains($title)) "stop.bat must close the $title terminal."
    Assert ($rootStart -match "(?im)^start\s+`"$([regex]::Escape($title))`"\s+cmd\s+/c\s+") "start.bat must launch the $title terminal with cmd /c."
}

foreach ($file in @(
    "windows-docker-nexus-log.cmd",
    "windows-docker-postgres-log.cmd",
    "windows-docker-redis-log.cmd"
)) {
    $logScript = Get-Content -LiteralPath (Join-Path $PSScriptRoot $file) -Raw
    Assert ($logScript -notmatch "(?im)^\s*pause\s*$") "$file must close automatically when its log stream ends."
}

Write-Host "Stop script terminal behavior checks passed."
