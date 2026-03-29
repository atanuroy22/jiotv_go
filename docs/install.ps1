try {
    $architecture = (Get-WmiObject Win32_OperatingSystem).OSArchitecture
    switch ($architecture) {
        "64-bit" { $arch = "amd64" }
        "32-bit" { $arch = "386" }
        "ARM64"  { $arch = "arm64" }
        default  { throw "Unsupported architecture: $architecture" }
    }

    Write-Host "Detected architecture: $arch"

    $homeDirectory = [System.IO.Path]::Combine($env:USERPROFILE, ".jiotv_go")
    if (-not (Test-Path $homeDirectory -PathType Container)) {
        New-Item -ItemType Directory -Force -Path $homeDirectory | Out-Null
    }
    Set-Location -Path $homeDirectory

    if (Test-Path jiotv_go.exe) {
        Write-Host "Deleting existing binary"
        Remove-Item jiotv_go.exe
    }

    $binaryUrl = "https://github.com/atanuroy22/jiotv_go/releases/latest/download/jiotv_go-windows-$arch.exe"
    Write-Host "Fetching the latest binary from $binaryUrl"
    Invoke-WebRequest -Uri $binaryUrl -OutFile jiotv_go.exe -UseBasicParsing

    Write-Host "JioTV Go has successfully downloaded. You can run it from the current folder. Start by running .\jiotv_go.exe help"
}
catch {
    Write-Host "Error: $_"
}
