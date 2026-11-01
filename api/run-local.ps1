$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $projectRoot '.env.local'
if (-not (Test-Path -LiteralPath $envFile)) {
    throw "Local settings file not found: $envFile"
}

$allowedNames = @(
    'DATABASE_URL', 'JWT_SECRET', 'HTTP_ADDR', 'COOKIE_SECURE',
    'GOOGLE_CLIENT_ID', 'GOOGLE_CLIENT_SECRET', 'GOOGLE_REDIRECT_URL', 'APP_BASE_URL', 'PUBLIC_LANDING_ONLY', 'PUBLIC_SHOP_ID',
    'SMTP_HOST', 'SMTP_PORT', 'SMTP_USERNAME', 'SMTP_PASSWORD', 'SMTP_FROM'
)
foreach ($line in Get-Content -LiteralPath $envFile) {
    $entry = $line.Trim()
    if (-not $entry -or $entry.StartsWith('#')) {
        continue
    }

    $separator = $entry.IndexOf('=')
    if ($separator -lt 1) {
        throw "Invalid setting in $envFile"
    }

    $name = $entry.Substring(0, $separator)
    if ($name -notin $allowedNames) {
        throw "Unsupported setting '$name' in $envFile"
    }
    Set-Item -Path "Env:$name" -Value $entry.Substring($separator + 1)
}

$databaseUri = [Uri]$env:DATABASE_URL
if ($databaseUri.Host -in @('127.0.0.1', 'localhost')) {
    $postgresRoot = Join-Path $env:ProgramFiles 'PostgreSQL'
    $installations = Get-ChildItem -LiteralPath $postgresRoot -Directory -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -match '^\d+$' -and (Test-Path (Join-Path $_.FullName 'bin\psql.exe')) } |
        Sort-Object { [int]$_.Name } -Descending
    if ($installations) {
        $postgresBin = Join-Path $installations[0].FullName 'bin'
        $psql = Join-Path $postgresBin 'psql.exe'
        $createdb = Join-Path $postgresBin 'createdb.exe'
        if ((Test-Path -LiteralPath $psql) -and (Test-Path -LiteralPath $createdb)) {
            $databasePassword = [Uri]::UnescapeDataString($databaseUri.UserInfo.Split(':', 2)[1])
            $env:PGPASSWORD = $databasePassword
            $databaseExists = & $psql -h $databaseUri.Host -p $databaseUri.Port -U postgres -d postgres -w -tAc "SELECT 1 FROM pg_database WHERE datname = 'cm_retailos_online'"
            if ($LASTEXITCODE -eq 0 -and (@($databaseExists) -join '').Trim() -ne '1') {
                & $createdb -h $databaseUri.Host -p $databaseUri.Port -U postgres -w cm_retailos_online
            }
        }
    }
}

Set-Location $PSScriptRoot
while ($true) {
    if (Test-Path (Join-Path $PSScriptRoot 'server.exe')) {
        & .\server.exe
    } else {
        go run ./cmd/server
    }
    Start-Sleep -Seconds 1
}
if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
}
