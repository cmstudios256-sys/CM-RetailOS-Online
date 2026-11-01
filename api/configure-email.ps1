$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $projectRoot '.env.local'
if (-not (Test-Path -LiteralPath $envFile)) {
    throw "Local settings file not found: $envFile. Run configure-local.ps1 first."
}

Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$form = New-Object System.Windows.Forms.Form
$form.Text = 'CM RetailOS Online - Brevo Email'
$form.Size = New-Object System.Drawing.Size(500, 365)
$form.StartPosition = 'CenterScreen'
$form.FormBorderStyle = 'FixedDialog'
$form.MaximizeBox = $false
$form.MinimizeBox = $false
$form.TopMost = $true

$appURLLabel = New-Object System.Windows.Forms.Label
$appURLLabel.Text = 'App origin (local default: http://localhost:5180):'
$appURLLabel.Location = New-Object System.Drawing.Point(16, 16)
$appURLLabel.AutoSize = $true
$form.Controls.Add($appURLLabel)

$appURLBox = New-Object System.Windows.Forms.TextBox
$appURLBox.Location = New-Object System.Drawing.Point(18, 38)
$appURLBox.Size = New-Object System.Drawing.Size(445, 24)
$appURLBox.Text = 'http://localhost:5180'
$form.Controls.Add($appURLBox)

$usernameLabel = New-Object System.Windows.Forms.Label
$usernameLabel.Text = 'Brevo SMTP login:'
$usernameLabel.Location = New-Object System.Drawing.Point(16, 76)
$usernameLabel.AutoSize = $true
$form.Controls.Add($usernameLabel)

$usernameBox = New-Object System.Windows.Forms.TextBox
$usernameBox.Location = New-Object System.Drawing.Point(18, 98)
$usernameBox.Size = New-Object System.Drawing.Size(445, 24)
$form.Controls.Add($usernameBox)

$passwordLabel = New-Object System.Windows.Forms.Label
$passwordLabel.Text = 'Brevo SMTP key (not your account password):'
$passwordLabel.Location = New-Object System.Drawing.Point(16, 136)
$passwordLabel.AutoSize = $true
$form.Controls.Add($passwordLabel)

$passwordBox = New-Object System.Windows.Forms.TextBox
$passwordBox.Location = New-Object System.Drawing.Point(18, 158)
$passwordBox.Size = New-Object System.Drawing.Size(445, 24)
$passwordBox.UseSystemPasswordChar = $true
$form.Controls.Add($passwordBox)

$senderLabel = New-Object System.Windows.Forms.Label
$senderLabel.Text = 'Verified sender email address in Brevo:'
$senderLabel.Location = New-Object System.Drawing.Point(16, 196)
$senderLabel.AutoSize = $true
$form.Controls.Add($senderLabel)

$senderBox = New-Object System.Windows.Forms.TextBox
$senderBox.Location = New-Object System.Drawing.Point(18, 218)
$senderBox.Size = New-Object System.Drawing.Size(445, 24)
$form.Controls.Add($senderBox)

$saveButton = New-Object System.Windows.Forms.Button
$saveButton.Text = 'Save locally'
$saveButton.Location = New-Object System.Drawing.Point(270, 270)
$saveButton.DialogResult = [System.Windows.Forms.DialogResult]::OK
$form.Controls.Add($saveButton)

$cancelButton = New-Object System.Windows.Forms.Button
$cancelButton.Text = 'Cancel'
$cancelButton.Location = New-Object System.Drawing.Point(360, 270)
$cancelButton.DialogResult = [System.Windows.Forms.DialogResult]::Cancel
$form.Controls.Add($cancelButton)
$form.AcceptButton = $saveButton
$form.CancelButton = $cancelButton

if ($form.ShowDialog() -ne [System.Windows.Forms.DialogResult]::OK) {
    $passwordBox.Clear()
    $form.Dispose()
    throw 'Brevo email setup was cancelled.'
}

$appURL = $appURLBox.Text.Trim().TrimEnd('/')
$smtpUsername = $usernameBox.Text.Trim()
$smtpPassword = $passwordBox.Text
$smtpFrom = $senderBox.Text.Trim()
$passwordBox.Clear()
$form.Dispose()

$parsedAppURL = $null
if (-not [Uri]::TryCreate($appURL, [UriKind]::Absolute, [ref]$parsedAppURL) -or
    $parsedAppURL.Scheme -notin @('http', 'https') -or
    $parsedAppURL.UserInfo -or $parsedAppURL.Query -or $parsedAppURL.Fragment -or
    $parsedAppURL.AbsolutePath -ne '/') {
    $smtpPassword = $null
    throw 'App origin must be an HTTP(S) URL without a path, query, or fragment.'
}
if ($parsedAppURL.Scheme -eq 'http' -and $parsedAppURL.Host -notin @('localhost', '127.0.0.1', '::1')) {
    $smtpPassword = $null
    throw 'Use HTTPS for the app origin outside local development.'
}
if ([string]::IsNullOrWhiteSpace($smtpUsername) -or [string]::IsNullOrWhiteSpace($smtpPassword) -or
    [string]::IsNullOrWhiteSpace($smtpFrom) -or $smtpFrom -notmatch '^[^@\s]+@[^@\s]+\.[^@\s]+$') {
    $smtpPassword = $null
    throw 'Enter the Brevo SMTP login, SMTP key, and a valid verified sender address.'
}

$settings = [ordered]@{
    APP_BASE_URL = $appURL
    SMTP_HOST = 'smtp-relay.brevo.com'
    SMTP_PORT = '587'
    SMTP_USERNAME = $smtpUsername
    SMTP_PASSWORD = $smtpPassword
    SMTP_FROM = $smtpFrom
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
$smtpPassword = $null
$settings.SMTP_PASSWORD = $null
Write-Output 'Brevo SMTP settings saved to the ignored local settings file. Restart the API to enable email signup verification and password recovery.'
