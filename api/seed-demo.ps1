param(
    [Parameter(Mandatory = $true)]
    [string]$DemoShopId
)

$ErrorActionPreference = "Stop"
$env:DEMO_SHOP_ID = $DemoShopId
go run ./cmd/seed-demo
