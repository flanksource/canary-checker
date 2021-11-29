 param (
    [string]$configfile = "$pwd\canary-checker.yaml",
    [int]$httpPort = 8080,
    [int]$metricsPort = 8081,
    [string]$name = "local",
    [switch]$uninstall = $false,
    [string]$pushServers
 )



Write-Host "Checking current priviledges"
If (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(`
[Security.Principal.WindowsBuiltInRole] "Administrator"))
{
    Write-Warning "You do not have Administrator rights to run this script!`nPlease re-run this script as an Administrator!"
    exit
}

Write-Host "Installing nssm for use"

if (!(Test-Path  ".\nssm-2.24\win64\nssm.exe"))
{
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
     Invoke-WebRequest -Uri  https://nssm.cc/release/nssm-2.24.zip -OutFile nssm-2.24.zip
     expand-archive -path '.\nssm-2.24.zip' -destinationpath '.'
}

if (!(Test-Path ".\canary-checker.exe") ) {
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker.exe -OutFile canary-checker.exe
}

$path="$pwd\canary-checker.exe"

if ($uninstall) {
    .\nssm-2.24\win64\nssm.exe stop canary-checker
    .\nssm-2.24\win64\nssm.exe remove canary-checker confirm
} else {
    .\nssm-2.24\win64\nssm.exe install canary-checker "$path"
    .\nssm-2.24\win64\nssm.exe set canary-checker AppParameters "serve --configfile $configfile --httpPort $httpPort --metricsPort $metricsPort --name $name --push-servers=$pushServers"
    .\nssm-2.24\win64\nssm.exe set canary-checker DisplayName "Canary Checker Server"
    .\nssm-2.24\win64\nssm.exe set canary-checker Description "Starts the Canary Checker Server"
    .\nssm-2.24\win64\nssm.exe start canary-checker
}
