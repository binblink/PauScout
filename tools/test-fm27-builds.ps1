<#
.SYNOPSIS
Runs PauScout's Windows regression tests against the two tested FM27 builds.
.EXAMPLE
./tools/test-fm27-builds.ps1 -OriginalSave ./original.fm -PatchedSave ./patched.fm
#>
[CmdletBinding()]
param(
    [Parameter(Mandatory)][string] $OriginalSave,
    [Parameter(Mandatory)][string] $PatchedSave,
    [string] $Go = 'go'
)

$ErrorActionPreference = 'Stop'
$cases = @(
    @{ Build = '27.0.1+2503222'; Path = (Resolve-Path -LiteralPath $OriginalSave).Path; PlayerID = '49054454' },
    @{ Build = '27.1.0+2505117'; Path = (Resolve-Path -LiteralPath $PatchedSave).Path; PlayerID = '34001981' }
)
$previousSample = $env:FM27_SAMPLE
$previousPlayer = $env:FM27_SAMPLE_PLAYER_ID
$previousBuild = $env:FM27_EXPECTED_BUILD
Push-Location (Join-Path $PSScriptRoot '..')
try {
    foreach ($case in $cases) {
        Write-Host "FM27 $($case.Build)"
        $env:FM27_SAMPLE = $case.Path
        $env:FM27_SAMPLE_PLAYER_ID = $case.PlayerID
        $env:FM27_EXPECTED_BUILD = $case.Build
        & $Go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) {
            throw "Regression tests failed for FM27 $($case.Build)"
        }
    }
} finally {
    $env:FM27_SAMPLE = $previousSample
    $env:FM27_SAMPLE_PLAYER_ID = $previousPlayer
    $env:FM27_EXPECTED_BUILD = $previousBuild
    Pop-Location
}
