$ErrorActionPreference = "Stop"

$repo = "SideNation/AICoordinator"
$installDir = "$env:LOCALAPPDATA\aico"
$exe = "$installDir\aico.exe"

New-Item -ItemType Directory -Force -Path $installDir | Out-Null

$url = "https://github.com/$repo/releases/latest/download/aico-windows-amd64.exe"
Write-Host "Downloading aico for windows/amd64..."
Invoke-WebRequest -Uri $url -OutFile $exe -UseBasicParsing

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$installDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$installDir;$userPath", "User")
    $env:Path = "$installDir;$env:Path"
    Write-Host "Added $installDir to PATH (restart shell to apply)"
}

Write-Host "Installed aico to $exe"
& $exe version
