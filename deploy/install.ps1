# NeoCP Professional - Windows Production Installer
# 🚀 The Master Ignition Sequence (PowerShell Edition)

Write-Host "----------------------------------------------------" -ForegroundColor Cyan
Write-Host "   NeoCP Professional - Windows Installation        " -ForegroundColor Cyan
Write-Host "----------------------------------------------------" -ForegroundColor Cyan

# 1. Administrator Check
$currentPrincipal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $currentPrincipal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    Write-Error "Please run this script as an Administrator."
    exit
}

# 2. Directory Setup
$InstallDir = "C:\Program Files\NeoCP"
if (-not (Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory
}

# 3. Binary Deployment
Write-Host "[1/4] Deploying NeoCP Monolithic Executable..." -ForegroundColor Yellow
$SourceBinary = "dist\neocp.exe"

if (Test-Path $SourceBinary) {
    Copy-Item $SourceBinary -Destination "$InstallDir\neocp.exe" -Force
} else {
    Write-Host "Notice: Local distribution not found. Simulation download..."
    # In production: Invoke-WebRequest -Uri "https://release.neocp.io/neocp.exe" -OutFile "$InstallDir\neocp.exe"
    # For simulation, try to find it in the repo
    $Found = Get-ChildItem -Recurse -Filter "neocp.exe" | Select-Object -First 1
    if ($Found) {
        Copy-Item $Found.FullName -Destination "$InstallDir\neocp.exe" -Force
    } else {
        Write-Error "NeoCP binary not found. Run 'make windows' first."
        exit
    }
}

# 4. Firewall Configuration
Write-Host "[2/4] Opening Firewall Ports (8443, 8080, 8444)..." -ForegroundColor Yellow
New-NetFirewallRule -DisplayName "NeoCP HTTPS" -Direction Inbound -LocalPort 8443 -Protocol TCP -Action Allow -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName "NeoCP HTTP Dev" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow -ErrorAction SilentlyContinue
New-NetFirewallRule -DisplayName "NeoCP Cluster mTLS" -Direction Inbound -LocalPort 8444 -Protocol TCP -Action Allow -ErrorAction SilentlyContinue

# 5. Windows Service Registration
Write-Host "[3/4] Registering Windows Background Service..." -ForegroundColor Yellow
$ServiceName = "NeoCPCore"
$ServiceDisplayName = "NeoCP Professional Core Service"

if (Get-Service -Name $ServiceName -ErrorAction SilentlyContinue) {
    Stop-Service -Name $ServiceName
    sc.exe delete $ServiceName
}

# Using New-Service for registration
New-Service -Name $ServiceName `
            -BinaryPathName "`"$InstallDir\neocp.exe`"" `
            -DisplayName $ServiceDisplayName `
            -Description "Monolithic zero-dependency NeoCP Professional hosting control panel daemon." `
            -StartupType Automatic

# 6. Finalizing
Write-Host "[4/4] Finalizing Installation..." -ForegroundColor Yellow
# Start-Service -Name $ServiceName

Write-Host "----------------------------------------------------" -ForegroundColor Green
Write-Host "🎉 NeoCP Windows Installation Complete!" -ForegroundColor Green
Write-Host "Dashboard: https://localhost:8443" -ForegroundColor Green
Write-Host "----------------------------------------------------" -ForegroundColor Green
