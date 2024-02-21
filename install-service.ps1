 param (
    [string]$configfile = "$pwd\canary-checker.yaml",
    [int]$httpPort = 8080,
    [string]$name = "local",

    [ValidateSet('install','reinstall','uninstall')]
    [System.String]$Operation = 'install'
 )

 if ([System.Environment]::OSVersion.Platform -inotmatch "Win32NT"){
    Write-Warning "This script is for windows OS only"
    exit -99
}
 if (($Operation -match "(re|un)install") -And $null -eq (get-service -Name canary-checker -ErrorAction SilentlyContinue)) {
    Write-Warning "Service does not exist, cannot perform $Operation, first try -operation install"
    exit -1
 } elseif (($Operation -eq "install") -And $null -ne (get-service -Name canary-checker -ErrorAction SilentlyContinue)) {
    Write-Warning "Service already installed, try '-operation uninstall' Or '-operation re-install'"
    exit -2
 }

Write-Host "Checking current priviledges"
If (-NOT ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(`
[Security.Principal.WindowsBuiltInRole] "Administrator"))
{
    Write-Warning "You do not have Administrator rights to run this script!`nPlease re-run this script as an Administrator!"
    exit -3
}


Write-Host "Installing nssm for use"

if (!(Test-Path  ".\nssm-2.24\win64\nssm.exe"))
{
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
     Invoke-WebRequest -Uri  https://nssm.cc/release/nssm-2.24.zip -OutFile nssm-2.24.zip
     expand-archive -path '.\nssm-2.24.zip' -destinationpath '.'
}

$scriptpath = $MyInvocation.MyCommand.Path
$dir = Split-Path $scriptpath
$existing_exe = "$dir\canary-checker.exe"

if ((Test-Path $existing_exe) -And !(Test-Path ".\canary-checker.exe")){
    Write-host "canary-checker exe found in this script path $existing_exe , using this exe version, no need to download from internet"
    if ($pwd -ne $dir ) {
        Write-Host "Current working path not the same as current script path, copying executable to here $pwd"
        copy-item $existing_exe $pwd
    }
} elseif (!(Test-Path ".\canary-checker.exe") ) {
    Write-host "canary-checker not found in $pwd, will attempt internet download"
    [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
    Invoke-WebRequest -Uri https://github.com/flanksource/canary-checker/releases/latest/download/canary-checker.exe -OutFile canary-checker.exe
}




if (!(Test-Path ".\\canary-checker.exe"))
{
    Write-Warning "Failed finding canary-checker executable"
    exit -4
}

if ( !(Test-Path $configfile)){
    Write-Warning "Failed finding canary-checker config file in $configfile, please specify a valid config file location"
    exit -5
}

if (!(Test-Path "$pwd\postgres-db")) {
    Write-Host "Creating folder for postgres data files - $pwd\postgres-db"
    New-Item -Path $pwd -Name "postgres-db" -ItemType "directory"
}
$dbpath = "$($pwd.path.Replace("\","/"))/postgres-db"


if ($Operation -match "((re|un)install)") {
    .\nssm-2.24\win64\nssm.exe stop canary-checker
    .\nssm-2.24\win64\nssm.exe remove canary-checker confirm
} 

if ($Operation -ne "uninstall") {
    .\nssm-2.24\win64\nssm.exe install canary-checker "$pwd\canary-checker.exe"
    .\nssm-2.24\win64\nssm.exe set canary-checker AppParameters "serve $configfile --httpPort $httpPort --name $name --db embedded://$dbpath --db-migrations"
    .\nssm-2.24\win64\nssm.exe set canary-checker DisplayName "Canary Checker Server"
    .\nssm-2.24\win64\nssm.exe set canary-checker Description "Starts the Canary Checker Server"
    .\nssm-2.24\win64\nssm.exe start canary-checker
}

exit 0