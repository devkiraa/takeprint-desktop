# Get or create code signing certificate
$subject = "CN=TakePrint Code Signing"
$cert = Get-ChildItem -Path Cert:\CurrentUser\My | Where-Object { $_.Subject -eq $subject } | Select-Object -First 1

if ($null -eq $cert) {
    Write-Host "Creating self-signed code signing certificate..."
    $cert = New-SelfSignedCertificate -Type CodeSigningCert -Subject $subject -KeyLength 2048 -NotAfter (Get-Date).AddYears(10) -CertStoreLocation "Cert:\CurrentUser\My"
}

# Export the public key certificate
$certPath = Join-Path $PSScriptRoot "takeprint.cer"
Write-Host "Exporting public certificate to $certPath..."
Export-Certificate -Cert $cert -FilePath $certPath -Force

# Helper to sign file
function Sign-File ($filePath) {
    if (Test-Path $filePath) {
        Write-Host "Signing $filePath..."
        Set-AuthenticodeSignature -FilePath $filePath -Certificate $cert
    } else {
        Write-Warning "File not found: $filePath"
    }
}

# Sign application binary and installers in build/bin
$binDir = Join-Path $PSScriptRoot "..\bin"
if (Test-Path $binDir) {
    Get-ChildItem -Path $binDir -Filter "*.exe" | ForEach-Object {
        Sign-File $_.FullName
    }
}

# Sign any built installers in build/windows/bin
$installerDir = Join-Path $PSScriptRoot "bin"
if (Test-Path $installerDir) {
    Get-ChildItem -Path $installerDir -Filter "*.exe" | ForEach-Object {
        Sign-File $_.FullName
    }
}
