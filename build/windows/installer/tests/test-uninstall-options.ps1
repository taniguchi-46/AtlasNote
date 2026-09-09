$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptRoot "..\..\..\.."))
$uninstallSourcePath = Join-Path $projectRoot "build\windows\installer\uninstall.nsh"
$productionSourcePath = Join-Path $projectRoot "build\windows\installer\project.nsi"
$harnessSourcePath = Join-Path $projectRoot "build\windows\installer\tests\uninstall-harness.nsi"
$harnessBinaryPath = Join-Path $projectRoot "build\windows\installer\uninstall-harness.exe"
$makensis = "C:\Program Files (x86)\NSIS\makensis.exe"
if (-not (Test-Path -LiteralPath $makensis)) {
    $makensis = (Get-Command makensis.exe -ErrorAction Stop).Source
}

function Assert-SourceContains {
    param(
        [Parameter(Mandatory = $true)][string]$Source,
        [Parameter(Mandatory = $true)][string]$Pattern,
        [Parameter(Mandatory = $true)][string]$Context
    )
    if ($Source -notmatch $Pattern) {
        throw "$($Context): expected pattern was not found: $Pattern"
    }
}

try {
    $utf8 = [System.Text.Encoding]::UTF8
    $uninstallSource = [System.IO.File]::ReadAllText($uninstallSourcePath, $utf8)
    $productionSource = [System.IO.File]::ReadAllText($productionSourcePath, $utf8)
    $harnessSourceText = [System.IO.File]::ReadAllText($harnessSourcePath, $utf8)

    Assert-SourceContains -Source $uninstallSource -Pattern '\u7AEF\u672B\u306E\u8868\u793A\u8A2D\u5B9A\u30FB\u30AD\u30E3\u30C3\u30B7\u30E5\u3092\u524A\u9664' -Context "display settings option"
    Assert-SourceContains -Source $uninstallSource -Pattern '\u3053\u306E\u5229\u7528\u8005\u306EAtlas Note\u7528\u8A8D\u8A3C\u60C5\u5831\u3092\u524A\u9664' -Context "credentials option"
    Assert-SourceContains -Source $uninstallSource -Pattern '\$AtlasNoteUninstallDeleteDisplaySettings 0' -Context "display settings default"
    Assert-SourceContains -Source $uninstallSource -Pattern '\$AtlasNoteUninstallDeleteCredentials 0' -Context "credentials default"
    Assert-SourceContains -Source $uninstallSource -Pattern '\$AtlasNoteUninstallDeleteDisplaySettings \$\{BST_UNCHECKED\}' -Context "display settings unchecked state"
    Assert-SourceContains -Source $uninstallSource -Pattern '\$AtlasNoteUninstallDeleteCredentials \$\{BST_UNCHECKED\}' -Context "credentials unchecked state"
    Assert-SourceContains -Source $uninstallSource -Pattern '--delete-display-settings' -Context "display settings maintenance flag"
    Assert-SourceContains -Source $uninstallSource -Pattern '--delete-credentials' -Context "credentials maintenance flag"
    Assert-SourceContains -Source $uninstallSource -Pattern '--registered-user' -Context "expected user maintenance flag"
    Assert-SourceContains -Source $uninstallSource -Pattern 'IfSilent atlasnote_uninstall_optional_cleanup_done' -Context "silent uninstall guard"
    Assert-SourceContains -Source $productionSource -Pattern 'UninstPage custom un\.AtlasNoteUninstallOptionsPageCreate un\.AtlasNoteUninstallOptionsPageLeave' -Context "production uninstall options page"
    if ($productionSource -match '\$USERNAME|WriteRegStr.*AtlasNoteInstallUser') { throw "Installer must not register an installation account as an actual user" }
    Assert-SourceContains -Source $harnessSourceText -Pattern 'UninstPage custom un\.AtlasNoteUninstallOptionsPageCreate un\.AtlasNoteUninstallOptionsPageLeave' -Context "harness uninstall options page"

    & $makensis $harnessSourcePath
    if ($LASTEXITCODE -ne 0) {
        throw "NSIS uninstall options harness compilation failed with exit code $LASTEXITCODE"
    }
    $fixtureRoot = Join-Path ([IO.Path]::GetTempPath()) ("AtlasNote-cleanup-dispatch-" + [guid]::NewGuid().ToString("N"))
    [IO.Directory]::CreateDirectory($fixtureRoot) | Out-Null
    try {
        & $makensis "/DFIXTURE_OUTPUT=$fixtureRoot\maintenance.exe" (Join-Path $scriptRoot "maintenance-fixture.nsi")
        if ($LASTEXITCODE -ne 0) { throw "Maintenance fixture compilation failed" }
        & $makensis "/DFIXTURE_OUTPUT=$fixtureRoot\dispatch.exe" (Join-Path $scriptRoot "cleanup-dispatch-harness.nsi")
        if ($LASTEXITCODE -ne 0) { throw "Dispatch harness compilation failed" }
        $env:ATLAS_NOTE_CLEANUP_FIXTURE = $fixtureRoot
        foreach ($choice in @(@(1,0), @(0,1), @(1,1))) {
            foreach ($helperExit in @(0,1)) {
                $env:ATLAS_NOTE_CLEANUP_FIXTURE_DISPLAY = [string]$choice[0]
                $env:ATLAS_NOTE_CLEANUP_FIXTURE_CREDENTIALS = [string]$choice[1]
                $env:ATLAS_NOTE_CLEANUP_FIXTURE_EXIT = [string]$helperExit
                $process = Start-Process -FilePath "$fixtureRoot\dispatch.exe" -WindowStyle Hidden -PassThru -Wait
                if ($process.ExitCode -ne $helperExit) { throw "Maintenance failure was not propagated" }
                $expected = "--atlasnote-maintenance --registered-user"
                if ($choice[0] -eq 1) { $expected += " --delete-display-settings" }
                if ($choice[1] -eq 1) { $expected += " --delete-credentials" }
                $actual = [IO.File]::ReadAllText("$fixtureRoot\arguments.txt")
                if ($actual -ne $expected) { throw "Incorrect maintenance arguments: $actual" }
            }
        }
        Remove-Item -LiteralPath "$fixtureRoot\maintenance.exe"
        $process = Start-Process -FilePath "$fixtureRoot\dispatch.exe" -WindowStyle Hidden -PassThru -Wait
        if ($process.ExitCode -eq 0) { throw "ExecWait failure was ignored" }
        Write-Output "PASS: actual shared NSIS maintenance dispatch, selected flags and failure propagation"
    } finally {
        $resolvedFixture = [IO.Path]::GetFullPath($fixtureRoot)
        $tempPrefix = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\') + '\'
        if (-not $resolvedFixture.StartsWith($tempPrefix, [StringComparison]::OrdinalIgnoreCase) -or
            -not ([IO.Path]::GetFileName($resolvedFixture).StartsWith('AtlasNote-cleanup-dispatch-'))) { throw "Unsafe fixture cleanup path" }
        Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
        Remove-Item Env:ATLAS_NOTE_CLEANUP_FIXTURE, Env:ATLAS_NOTE_CLEANUP_FIXTURE_DISPLAY, Env:ATLAS_NOTE_CLEANUP_FIXTURE_CREDENTIALS, Env:ATLAS_NOTE_CLEANUP_FIXTURE_EXIT -ErrorAction SilentlyContinue
    }
    $env:GOCACHE = Join-Path ([IO.Path]::GetTempPath()) "atlasnote-review-gocache"
    Push-Location $projectRoot
    try {
        & go test ./internal/appcleanup -run 'TestRegisteredUserMaintenanceFlow|TestMaintenanceRequiresRegisteredIdentityAndExactFlags' -count=1
        if ($LASTEXITCODE -ne 0) { throw "SID registration-to-maintenance behavior regression failed" }
    } finally { Pop-Location }
    Write-Output "PASS: optional deletion selection wiring"
    Write-Output "Uninstall options harness tests passed."
} finally {
    if (Test-Path -LiteralPath $harnessBinaryPath) {
        Remove-Item -LiteralPath $harnessBinaryPath -Force
    }
}
