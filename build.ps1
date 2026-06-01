# Full rebuild: frontend + embed into Go binary (Windows)
$ErrorActionPreference = "Stop"
$root = $PSScriptRoot

Write-Host "Building frontend..."
Set-Location "$root\frontend"
npm install
npm run build

Write-Host "Copying to internal/api/static..."
Remove-Item -Recurse -Force "$root\internal\api\static\*" -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path "$root\internal\api\static" | Out-Null
Copy-Item -Recurse -Force "$root\frontend\dist\*" "$root\internal\api\static\"

Write-Host "Building Go binary..."
Set-Location $root
go build -o monitor.exe ./cmd/server

Write-Host "Done. Run: .\monitor.exe -db .\data.db"
