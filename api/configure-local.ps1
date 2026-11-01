$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $projectRoot '.env.local'
$postgresRoot = Join-Path $env:ProgramFiles 'PostgreSQL'
$installations = Get-ChildItem -LiteralPath $postgresRoot -Directory |
    Where-Object { $_.Name -match '^\d+$' -and (Test-Path (Join-Path $_.FullName 'bin\psql.exe')) } |
    Sort-Object { [int]$_.Name } -Descending
if (-not $installations) {
    throw "PostgreSQL command-line tools were not found in $postgresRoot"
}

$postgresBin = Join-Path $installations[0].FullName 'bin'
$psql = Join-Path $postgresBin 'psql.exe'
$createdb = Join-Path $postgresBin 'createdb.exe'

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$form = New-Object System.Windows.Forms.Form
$form.Text = 'CM RetailOS Online - Local Database'
$form.Size = New-Object System.Drawing.Size(440, 175)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.TopMost = $true

$label = New-Object System.Windows.Forms.Label
$label.Text = 'Enter the PostgreSQL postgres-user password:'
$label.Location = New-Object System.Drawing.Point(16, 18)
$label.AutoSize = $true
$form.Controls.Add($label)

$passwordBox = New-Object System.Windows.Forms.TextBox
$passwordBox.Location = New-Object System.Drawing.Point(18, 46)
$passwordBox.Size = New-Object System.Drawing.Size(390, 24)
$passwordBox.UseSystemPasswordChar = $true
$form.Controls.Add($passwordBox)

$continueButton = New-Object System.Windows.Forms.Button
$continueButton.Text = 'Connect'
$continueButton.Location = New-Object System.Drawing.Point(238, 88)
$continueButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
$form.Controls.Add($continueButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = 'Cancel'
$cancelButton.Location = New-Object System.Drawing.Point(328, 88)
$cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$form.Controls.Add($cancelButton)
$form.AcceptButton = $continueButton
$form.CancelButton = $cancelButton

if ($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) {
    $form.Dispose()
    throw 'Local database setup was cancelled.'
}
$password = $passwordBox.Text
$passwordBox.Clear()
$form.Dispose()
if ([string]::IsNullOrEmpty($password)) {
    throw 'The PostgreSQL password cannot be empty.'
}

$env:PGPASSWORD = $password
try {
    $databaseExists = & $psql -h 127.0.0.1 -p 5432 -U postgres -d postgres -w -tAc "SELECT 1 FROM pg_database WHERE datname = 'cm_retailos_online'"
    if ($LASTEXITCODE -ne 0) {
        throw 'Could not authenticate to PostgreSQL on 127.0.0.1:5432. Confirm the service is running and retry with the password you set during installation.'
    }

    if ((@($databaseExists) -join '').Trim() -ne '1') {
        & $createdb -h 127.0.0.1 -p 5432 -U postgres -w cm_retailos_online
        if ($LASTEXITCODE -ne 0) {
            throw 'Could not create the cm_retailos_online database.'
        }
    }

    $lines = @()
    if (Test-Path -LiteralPath $envFile) {
        $lines = @(Get-Content -LiteralPath $envFile | Where-Object { $_ -notmatch '^DATABASE_URL=' })
    }
    $escapedPassword = [Uri]::EscapeDataString($password)
    $databaseUrl = "DATABASE_URL=postgres://postgres:${escapedPassword}@127.0.0.1:5432/cm_retailos_online?sslmode=disable"

    Set-Content -LiteralPath $envFile -Value (@($databaseUrl) + $lines) -Encoding ascii
    Write-Output 'Local database connection configured and development database is ready.'
}
finally {
    Remove-Item Env:PGPASSWORD -ErrorAction SilentlyContinue
    $password = $null
}
