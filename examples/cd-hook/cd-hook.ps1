# find-uncommitted — PowerShell cd hook (Fork A ambient pre-flight)
#
# Dot-source from your PowerShell profile ($PROFILE):
#   . "C:\path\to\find-uncommitted\examples\cd-hook\cd-hook.ps1"
#
# Env:
#   $env:FIND_UNCOMMITTED_BIN      Path to find-uncommitted.exe
#   $env:FIND_UNCOMMITTED_CD_ARGS  Extra args before "check" (e.g. --no-remote)
#   $env:FIND_UNCOMMITTED_CD_HOOK=0  Disable without unsourcing
#
# Wraps the `cd` function (common interactive use). Push-Location / Pop-Location
# are also wrapped. Prefer `cd` for day-to-day navigation.

$script:_FindUncommittedLastTop = $null

function Get-FindUncommittedBin {
    if ($env:FIND_UNCOMMITTED_BIN) {
        return $env:FIND_UNCOMMITTED_BIN
    }
    $cmd = Get-Command find-uncommitted.exe -ErrorAction SilentlyContinue
    if (-not $cmd) {
        $cmd = Get-Command find-uncommitted -ErrorAction SilentlyContinue
    }
    if ($cmd) {
        return $cmd.Source
    }
    return $null
}

function Invoke-FindUncommittedCdCheck {
    if ($env:FIND_UNCOMMITTED_CD_HOOK -eq '0') {
        return
    }

    $bin = Get-FindUncommittedBin
    if (-not $bin -or -not (Test-Path -LiteralPath $bin)) {
        return
    }

    $pwdPath = (Get-Location).Path
    if (-not (Test-Path -LiteralPath $pwdPath -PathType Container)) {
        $script:_FindUncommittedLastTop = $null
        return
    }

    # Leaving a work tree clears the cache so re-entry runs check again.
    & git -C $pwdPath rev-parse --is-inside-work-tree 2>$null | Out-Null
    if ($LASTEXITCODE -ne 0) {
        $script:_FindUncommittedLastTop = $null
        return
    }

    $top = (& git -C $pwdPath rev-parse --show-toplevel 2>$null | Out-String).Trim()
    if (-not $top) {
        $script:_FindUncommittedLastTop = $null
        return
    }
    if ($script:_FindUncommittedLastTop -eq $top) {
        return
    }
    $script:_FindUncommittedLastTop = $top

    $extra = @()
    if ($env:FIND_UNCOMMITTED_CD_ARGS) {
        $extra = @(
            $env:FIND_UNCOMMITTED_CD_ARGS -split '\s+' | Where-Object { $_ }
        )
    }

    $prevEap = $ErrorActionPreference
    $ErrorActionPreference = 'Continue'
    try {
        $out = & $bin @extra check $pwdPath 2>$null
        $ec = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $prevEap
    }

    if ($ec -eq 2 -and $out) {
        if ($out -is [array]) {
            $out -join [Environment]::NewLine
        } else {
            Write-Output $out
        }
    }
}

function cd {
    [CmdletBinding()]
    param(
        [Parameter(Position = 0, ValueFromPipeline = $true, ValueFromPipelineByPropertyName = $true)]
        [string] $Path,

        [Parameter(ValueFromRemainingArguments = $true)]
        [object[]] $RemainingArguments
    )

    if ($PSBoundParameters.ContainsKey('Path') -and $Path -ne '') {
        Microsoft.PowerShell.Management\Set-Location -Path $Path @RemainingArguments
    } elseif ($RemainingArguments -and $RemainingArguments.Count -gt 0) {
        Microsoft.PowerShell.Management\Set-Location @RemainingArguments
    } else {
        Microsoft.PowerShell.Management\Set-Location
    }

    if ($?) {
        Invoke-FindUncommittedCdCheck
    }
}

function Push-Location {
    [CmdletBinding()]
    param(
        [Parameter(Position = 0)]
        [string] $Path,

        [Parameter(ValueFromRemainingArguments = $true)]
        [object[]] $RemainingArguments
    )

    if ($PSBoundParameters.ContainsKey('Path') -and $Path -ne '') {
        Microsoft.PowerShell.Management\Push-Location -Path $Path @RemainingArguments
    } else {
        Microsoft.PowerShell.Management\Push-Location @RemainingArguments
    }

    if ($?) {
        Invoke-FindUncommittedCdCheck
    }
}

function Pop-Location {
    [CmdletBinding()]
    param(
        [Parameter(ValueFromRemainingArguments = $true)]
        [object[]] $RemainingArguments
    )

    Microsoft.PowerShell.Management\Pop-Location @RemainingArguments
    if ($?) {
        Invoke-FindUncommittedCdCheck
    }
}
