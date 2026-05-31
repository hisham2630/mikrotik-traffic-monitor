# Windows build script
Set-Location $PSScriptRoot
Write-Host "Building frontend..."
Set-Location frontend
npm install
npm run build
Set-Location ..
$staticDir = "internal\api\static"
if (Test-Path $staticDir) { Remove-Item -Recurse -Force $staticDir }
New-Item -ItemType Directory -Path $staticDir | Out-Null
Copy-Item -Recurse frontend\dist\* $staticDir\
Write-Host "Building Go binary..."
go mod tidy
go build -o mikrotik-monitor.exe ./cmd/server
Write-Host "Done: mikrotik-monitor.exe"
