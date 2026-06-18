# probaci installer for Windows — downloads the right release binary, verifies
# its checksum, and installs it onto your PATH.
#
#   irm https://raw.githubusercontent.com/simtabi/probaci/main/install.ps1 | iex
#
# Environment overrides:
#   PROBACI_VERSION       version to install (default: latest release)
#   PROBACI_INSTALL_DIR   install directory (default: %LOCALAPPDATA%\Programs\probaci,
#                         or %ProgramFiles%\probaci when running elevated)
#   PROBACI_BASE_URL      base URL for the archive + checksums.txt (default: the
#                         GitHub release download dir; set to a local dir to test
#                         a locally-built bundle)
#requires -version 5
$ErrorActionPreference = 'Stop'

$Repo = 'simtabi/probaci'
$Bin  = 'probaci'

function Info($m) { Write-Host "==> $m" }
function Die($m)  { Write-Error $m; exit 1 }

function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        'AMD64' { 'amd64' }
        'ARM64' { 'arm64' }
        'x86'   { '386' }
        default { Die "unsupported arch: $($env:PROCESSOR_ARCHITECTURE)" }
    }
}

function Resolve-Version {
    if ($env:PROBACI_VERSION) { return ($env:PROBACI_VERSION -replace '^v', '') }
    if ($env:PROBACI_BASE_URL) { Die 'set PROBACI_VERSION when PROBACI_BASE_URL is overridden' }
    Info 'resolving latest release'
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return ($rel.tag_name -replace '^v', '')
}

function Resolve-Dir {
    if ($env:PROBACI_INSTALL_DIR) { return $env:PROBACI_INSTALL_DIR }
    $elevated = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()
        ).IsInRole([Security.Principal.WindowsBuiltinRole]::Administrator)
    if ($elevated) { return (Join-Path $env:ProgramFiles 'probaci') }
    return (Join-Path $env:LOCALAPPDATA 'Programs\probaci')
}

$ver  = Resolve-Version
$tag  = "v$ver"
$arch = Get-Arch
$asset = "${Bin}_${ver}_windows_${arch}.zip"
$base = if ($env:PROBACI_BASE_URL) { $env:PROBACI_BASE_URL.TrimEnd('/') } else {
    "https://github.com/$Repo/releases/download/$tag"
}

$work = Join-Path ([IO.Path]::GetTempPath()) ("probaci-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $work | Out-Null
try {
    Info "downloading $asset"
    $zip = Join-Path $work $asset
    Invoke-WebRequest -Uri "$base/$asset" -OutFile $zip

    try {
        $sums = Join-Path $work 'checksums.txt'
        Invoke-WebRequest -Uri "$base/checksums.txt" -OutFile $sums
        $want = (Select-String -Path $sums -Pattern ([Regex]::Escape($asset)) | Select-Object -First 1).Line
        if ($want) {
            $wantHash = ($want -split '\s+')[0]
            $gotHash = (Get-FileHash -Algorithm SHA256 $zip).Hash.ToLower()
            if ($wantHash.ToLower() -ne $gotHash) { Die "checksum mismatch for $asset" }
            Info 'checksum verified'
        }
    } catch { Write-Warning 'could not verify checksum (continuing)' }

    Info 'extracting'
    Expand-Archive -Path $zip -DestinationPath $work -Force
    $exe = Join-Path $work "$Bin.exe"
    if (-not (Test-Path $exe)) { Die "archive did not contain $Bin.exe" }

    $dir = Resolve-Dir
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    Copy-Item $exe (Join-Path $dir "$Bin.exe") -Force
    Info "installed $Bin $ver to $dir\$Bin.exe"

    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dir) {
        Write-Warning "$dir is not on your PATH. Add it with:"
        Write-Host "  [Environment]::SetEnvironmentVariable('Path', `"$dir;`$env:Path`", 'User')"
    }
} finally {
    Remove-Item -Recurse -Force $work -ErrorAction SilentlyContinue
}
