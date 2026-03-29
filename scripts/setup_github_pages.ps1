param(
  [string]$Owner = "atanuroy22",
  [string]$Repo = "jiotv_go",
  [string]$Branch = "gh-pages",
  [switch]$Uninstall,
  [string]$TokenEnvVar = "GITHUB_TOKEN"
)
$ErrorActionPreference = "Stop"
function Fail($m) { Write-Host $m; exit 1 }
$token = [Environment]::GetEnvironmentVariable($TokenEnvVar)
if ([string]::IsNullOrWhiteSpace($token)) { Fail "Missing token in $TokenEnvVar" }
$remote = "https://$token@github.com/$Owner/$Repo.git"
if ($Uninstall) {
  try {
    $h = @{ Authorization = "token $token"; Accept = "application/vnd.github+json" }
    Invoke-RestMethod -Method Delete -Headers $h -Uri "https://api.github.com/repos/$Owner/$Repo/pages" -ErrorAction SilentlyContinue | Out-Null
  } catch {}
  try {
    git init
    git remote add origin $remote
  } catch {}
  try {
    git push origin --delete $Branch
  } catch {}
  Write-Host "Unlinked GitHub Pages"
  exit 0
}
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Fail "git not found" }
$buildDir = Join-Path $env:TEMP ("jiotv_go_pages_" + [guid]::NewGuid().ToString())
New-Item -ItemType Directory -Force -Path $buildDir | Out-Null
$mdbookAvailable = Get-Command mdbook -ErrorAction SilentlyContinue
if ($mdbookAvailable) {
  mdbook build "docs"
  Copy-Item -Recurse -Force "docs/book/*" $buildDir
} else {
  $index = @"
<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>JioTV Go</title></head><body><h1>JioTV Go</h1><p>Documentation site</p><p><a href="./get_started.html">Get Started</a></p></body></html>
"@
  $null = New-Item -ItemType File -Path (Join-Path $buildDir "index.html") -Force
  Set-Content -Path (Join-Path $buildDir "index.html") -Value $index -Encoding UTF8
  Copy-Item -Recurse -Force "docs/*" $buildDir
}
Copy-Item -Force "scripts/install.sh" (Join-Path $buildDir "install.sh")
Copy-Item -Force "scripts/install.ps1" (Join-Path $buildDir "install.ps1")
$nojekyll = Join-Path $buildDir ".nojekyll"
$null = New-Item -ItemType File -Path $nojekyll -Force
Push-Location $buildDir
git init
git checkout -b $Branch
git add -A
git commit -m "Deploy Pages"
git remote add origin $remote
git push -f origin $Branch
Pop-Location
$siteUrl = "https://$Owner.github.io/$Repo/"
$ok = $false
for ($i=0; $i -lt 30; $i++) {
  Start-Sleep -Seconds 5
  try {
    $resp = Invoke-WebRequest -Uri $siteUrl -UseBasicParsing
    if ($resp.StatusCode -eq 200 -and $resp.Content -match "JioTV Go") { $ok = $true; break }
  } catch {}
}
if (-not $ok) { Fail "Deployment verification failed" }
Write-Host "Deployment successful: $siteUrl"
