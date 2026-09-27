# Smoke-test examples/cd-hook/cd-hook.ps1 against a built binary.
$ErrorActionPreference = 'Stop'

$Root = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$Bin = if ($env:FIND_UNCOMMITTED_BIN) {
    $env:FIND_UNCOMMITTED_BIN
} else {
    Join-Path $Root 'binaries/find-uncommitted.exe'
}
if (-not (Test-Path -LiteralPath $Bin)) {
    $alt = Join-Path $Root 'binaries/find-uncommitted'
    if (Test-Path -LiteralPath $alt) { $Bin = $alt }
}

if (-not (Test-Path -LiteralPath $Bin)) {
    Write-Host "Building $Bin ..."
    New-Item -ItemType Directory -Force -Path (Join-Path $Root 'binaries') | Out-Null
    Push-Location $Root
    try {
        go build -o $Bin .
    } finally {
        Pop-Location
    }
}

$Hook = Join-Path $Root 'examples/cd-hook/cd-hook.ps1'
$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("fu-cd-hook-" + [guid]::NewGuid().ToString('n'))
New-Item -ItemType Directory -Path $Tmp | Out-Null

try {
    $Dirty = Join-Path $Tmp 'dirty'
    New-Item -ItemType Directory -Path $Dirty | Out-Null
    git -C $Dirty init -q
    git -C $Dirty config user.email 'test@example.com'
    git -C $Dirty config user.name 'Test'
    Set-Content -Path (Join-Path $Dirty 'file.txt') -Value 'hi'

    # Empty repo (no commits) → check exits 0 → hook silent.
    # (A committed repo without upstream is Attention: untracked-upstream.)
    $Clean = Join-Path $Tmp 'clean'
    New-Item -ItemType Directory -Path $Clean | Out-Null
    git -C $Clean init -q

    $NonGit = Join-Path $Tmp 'nongit'
    New-Item -ItemType Directory -Path $NonGit | Out-Null

    $env:FIND_UNCOMMITTED_BIN = $Bin
    $env:FIND_UNCOMMITTED_CD_ARGS = '--no-remote'
    $env:FIND_UNCOMMITTED_CD_HOOK = '1'

    function Invoke-HookCase {
        param(
            [string] $Label,
            [string] $Target,
            [ValidateSet('yes', 'no')]
            [string] $ExpectOutput
        )

        # Isolated child: call `cd` (what the hook wraps), not Set-Location.
        $script = @"
`$env:FIND_UNCOMMITTED_BIN = '$($Bin -replace '\\','\\')'
`$env:FIND_UNCOMMITTED_CD_ARGS = '--no-remote'
`$env:FIND_UNCOMMITTED_CD_HOOK = '1'
. '$($Hook -replace '\\','\\')'
cd '$($Target -replace '\\','\\')'
"@
        $out = & pwsh -NoProfile -Command $script 2>&1 | Out-String
        $out = $out.Trim()

        if ($ExpectOutput -eq 'yes') {
            if (-not $out) {
                throw "FAIL: $Label — expected Attention output, got silence"
            }
            Write-Host "OK:   $Label — printed Attention"
        } else {
            if ($out) {
                throw "FAIL: $Label — expected silence, got:`n$out"
            }
            Write-Host "OK:   $Label — silent"
        }
    }

    Write-Host "Binary: $Bin"
    Invoke-HookCase -Label 'non-git directory' -Target $NonGit -ExpectOutput no
    Invoke-HookCase -Label 'clean git repo' -Target $Clean -ExpectOutput no
    Invoke-HookCase -Label 'dirty git repo' -Target $Dirty -ExpectOutput yes

    $env:FIND_UNCOMMITTED_CD_HOOK = '0'
    $scriptOff = @"
`$env:FIND_UNCOMMITTED_BIN = '$($Bin -replace '\\','\\')'
`$env:FIND_UNCOMMITTED_CD_ARGS = '--no-remote'
`$env:FIND_UNCOMMITTED_CD_HOOK = '0'
. '$($Hook -replace '\\','\\')'
cd '$($Dirty -replace '\\','\\')'
"@
    $outOff = & pwsh -NoProfile -Command $scriptOff 2>&1 | Out-String
    if ($outOff.Trim()) {
        throw "FAIL: FIND_UNCOMMITTED_CD_HOOK=0 should silence"
    }
    Write-Host "OK:   FIND_UNCOMMITTED_CD_HOOK=0 — silent"

    # Leave work tree then re-enter — cache must clear so Attention prints again.
    $scriptReenter = @"
`$env:FIND_UNCOMMITTED_BIN = '$($Bin -replace '\\','\\')'
`$env:FIND_UNCOMMITTED_CD_ARGS = '--no-remote'
`$env:FIND_UNCOMMITTED_CD_HOOK = '1'
. '$($Hook -replace '\\','\\')'
cd '$($Dirty -replace '\\','\\')' | Out-Null
cd '$($NonGit -replace '\\','\\')' | Out-Null
cd '$($Dirty -replace '\\','\\')'
"@
    $outReenter = & pwsh -NoProfile -Command $scriptReenter 2>&1 | Out-String
    if (-not $outReenter.Trim()) {
        throw "FAIL: re-enter after leave — expected Attention output"
    }
    Write-Host "OK:   leave then re-enter — printed Attention"

    Write-Host 'All cd-hook PowerShell smoke tests passed.'
} finally {
    Remove-Item -Recurse -Force -LiteralPath $Tmp -ErrorAction SilentlyContinue
}
