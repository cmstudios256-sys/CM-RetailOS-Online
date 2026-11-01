$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $projectRoot '.env.local'
if (-not (Test-Path -LiteralPath $envFile)) {
    throw "Local settings file not found: $envFile"
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$form = New-Object System.Windows.Forms.Form
$form.Text = 'CM RetailOS Online - Google OAuth'
$form.Size = New-Object System.Drawing.Size(460, 245)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.TopMost = $true

$clientIDLabel = New-Object System.Windows.Forms.Label
$clientIDLabel.Text = 'Google OAuth Client ID:'
$clientIDLabel.Location = New-Object System.Drawing.Point(16, 16)
$clientIDLabel.AutoSize = $true
$form.Controls.Add($clientIDLabel)

$clientIDBox = New-Object System.Windows.Forms.TextBox
$clientIDBox.Location = New-Object System.Drawing.Point(18, 38)
$clientIDBox.Size = New-Object System.Drawing.Size(410, 24)
$form.Controls.Add($clientIDBox)

$secretLabel = New-Object System.Windows.Forms.Label
$secretLabel.Text = 'Google OAuth Client Secret:'
$secretLabel.Location = New-Object System.Drawing.Point(16, 73)
$secretLabel.AutoSize = $true
$form.Controls.Add($secretLabel)

$secretBox = New-Object System.Windows.Forms.TextBox
$secretBox.Location = New-Object System.Drawing.Point(18, 95)
$secretBox.Size = New-Object System.Drawing.Size(410, 24)
$secretBox.UseSystemPasswordChar = $true
$form.Controls.Add($secretBox)

$saveButton = New-Object System.Windows.Forms.Button
$saveButton.Text = 'Save locally'
$saveButton.Location = New-Object System.Drawing.Point(238, 145)
$saveButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
$form.Controls.Add($saveButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = 'Cancel'
$cancelButton.Location = New-Object System.Drawing.Point(328, 145)
$cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$form.Controls.Add($cancelButton)
$form.AcceptButton = $saveButton
$form.CancelButton = $cancelButton

if ($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) {
    $clientIDBox.Clear()
    $secretBox.Clear()
    $form.Dispose()
    throw 'Google OAuth setup was cancelled.'
}
$clientID = $clientIDBox.Text.Trim()
$clientSecret = $secretBox.Text
$clientIDBox.Clear()
$secretBox.Clear()
$form.Dispose()
if (-not $clientID.EndsWith('.apps.googleusercontent.com', [StringComparison]::OrdinalIgnoreCase)) {
    $clientSecret = $null
    throw 'The Google OAuth Client ID does not have the expected Google client ID format.'
}
if ([string]::IsNullOrWhiteSpace($clientSecret)) {
    throw 'Google OAuth Client Secret cannot be empty.'
}

$settings = [ordered]@{
    GOOGLE_CLIENT_ID = $clientID
    GOOGLE_CLIENT_SECRET = $clientSecret
    GOOGLE_REDIRECT_URL = 'http://localhost:8080/api/v1/auth/google/callback'
    APP_BASE_URL = 'http://localhost:5180'
}
$settingNames = @($settings.Keys)
$lines = @(Get-Content -LiteralPath $envFile | Where-Object {
    $line = $_
    -not ($settingNames | Where-Object { $line.StartsWith("$_=") })
})
foreach ($name in $settings.Keys) {
    $lines += "$name=$($settings[$name])"
}
Set-Content -LiteralPath $envFile -Value $lines -Encoding ascii
$clientSecret = $null
$settings.GOOGLE_CLIENT_SECRET = $null
Write-Output 'Google OAuth credentials saved to the ignored local settings file. Restart the API to enable Google sign-in.'
