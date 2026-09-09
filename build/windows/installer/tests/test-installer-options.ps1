$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptRoot "..\..\..\.."))
$harnessSource = Join-Path $projectRoot "build\windows\installer\tests\installer-options-harness.nsi"
$harnessBinary = Join-Path $projectRoot "build\windows\installer\installer-options-harness.exe"
$productionSource = Join-Path $projectRoot "build\windows\installer\project.nsi"
$optionsSource = Join-Path $projectRoot "build\windows\installer\installer-options.nsh"
$makensis = "C:\Program Files (x86)\NSIS\makensis.exe"
if (-not (Test-Path -LiteralPath $makensis)) {
    $makensis = (Get-Command makensis.exe -ErrorAction Stop).Source
}

$runnerRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("AtlasNote-installer-options-tests-" + [guid]::NewGuid().ToString("N"))
$environmentNames = @(
    "ATLAS_NOTE_OPTIONS_FIXTURE",
    "ATLAS_NOTE_OPTIONS_INSTALL_DIR",
    "ATLAS_NOTE_OPTIONS_START_SHORTCUT",
    "ATLAS_NOTE_OPTIONS_DESKTOP_SHORTCUT",
    "ATLAS_NOTE_OPTIONS_DESKTOP",
    "ATLAS_NOTE_OPTIONS_START",
    "ATLAS_NOTE_OPTIONS_FINISH_LAUNCH",
    "ATLAS_NOTE_OPTIONS_LAUNCH_MARKER"
)
$savedEnvironment = @{}
foreach ($name in $environmentNames) {
    $savedEnvironment[$name] = [System.Environment]::GetEnvironmentVariable($name, "Process")
}

function Assert-UnderRoot {
    param(
        [Parameter(Mandatory = $true)][string]$Path,
        [Parameter(Mandatory = $true)][string]$Root
    )
    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $fullRoot = [System.IO.Path]::GetFullPath($Root).TrimEnd([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar)
    $rootPrefix = $fullRoot + [System.IO.Path]::DirectorySeparatorChar
    if (($fullPath -ne $fullRoot) -and (-not $fullPath.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase))) {
        throw "Refusing to use a path outside the fixture root: $fullPath"
    }
    return $fullPath
}

function Remove-FixtureSafely {
    param([Parameter(Mandatory = $true)][string]$Root)
    $safeRoot = Assert-UnderRoot -Path $Root -Root $Root
    if (Test-Path -LiteralPath $safeRoot) {
        Remove-Item -LiteralPath $safeRoot -Recurse -Force
    }
}

function Set-OptionEnvironment {
    param(
        [Parameter(Mandatory = $true)]$Fixture,
        [string]$Desktop,
        [string]$Start,
        [string]$FinishLaunch,
        [string]$LaunchMarker
    )
    $values = @{
        "ATLAS_NOTE_OPTIONS_FIXTURE" = $Fixture.Root
        "ATLAS_NOTE_OPTIONS_INSTALL_DIR" = $Fixture.InstallDir
        "ATLAS_NOTE_OPTIONS_START_SHORTCUT" = $Fixture.StartShortcut
        "ATLAS_NOTE_OPTIONS_DESKTOP_SHORTCUT" = $Fixture.DesktopShortcut
        "ATLAS_NOTE_OPTIONS_DESKTOP" = $Desktop
        "ATLAS_NOTE_OPTIONS_START" = $Start
        "ATLAS_NOTE_OPTIONS_FINISH_LAUNCH" = $FinishLaunch
        "ATLAS_NOTE_OPTIONS_LAUNCH_MARKER" = $LaunchMarker
    }
    foreach ($entry in $values.GetEnumerator()) {
        [System.Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, "Process")
    }
}

function Invoke-HarnessInstaller {
    param([switch]$Silent)
    $arguments = if ($Silent) { @("/S") } else { @() }
    $process = Start-Process -FilePath $harnessBinary -ArgumentList $arguments -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -Wait
    if ($process.ExitCode -ne 0) {
        throw "Options harness failed with exit code $($process.ExitCode)"
    }
}

function New-Fixture {
    param([Parameter(Mandatory = $true)][string]$Name, [switch]$CustomInstallPath)
    $root = Join-Path $runnerRoot $Name
    $installDir = if ($CustomInstallPath) { Join-Path $root "custom-install" } else { Join-Path $root "install" }
    $defaultInstallDir = Join-Path $root "default-install"
    $fixture = [pscustomobject]@{
        Root = $root
        InstallDir = $installDir
        DefaultInstallDir = $defaultInstallDir
        StartShortcut = Join-Path $root "shortcuts\start.lnk"
        DesktopShortcut = Join-Path $root "shortcuts\desktop.lnk"
        LaunchMarker = Join-Path $root "finish-launch.marker"
    }
    New-Item -ItemType Directory -Path $fixture.Root -Force | Out-Null
    New-Item -ItemType Directory -Path $fixture.DefaultInstallDir -Force | Out-Null
    New-Item -ItemType Directory -Path (Split-Path -Parent $fixture.StartShortcut) -Force | Out-Null
    [System.IO.File]::WriteAllText((Join-Path $fixture.DefaultInstallDir "untouched.txt"), "default install sentinel")
    return $fixture
}

function Assert-State {
    param(
        [Parameter(Mandatory = $true)]$Fixture,
        [Parameter(Mandatory = $true)][string]$Desktop,
        [Parameter(Mandatory = $true)][string]$Start,
        [string]$Context = "installer options"
    )
    $statePath = Join-Path $Fixture.Root "options.ini"
    if (-not (Test-Path -LiteralPath $statePath)) { throw "$($Context): options state was not written" }
    $state = Get-Content -Raw -Encoding utf8 -LiteralPath $statePath
    if ($state -notmatch "desktop=$Desktop") { throw "$($Context): desktop state was not $Desktop" }
    if ($state -notmatch "start=$Start") { throw "$($Context): start state was not $Start" }
    if ($Desktop -eq "1") {
        if (-not (Test-Path -LiteralPath $Fixture.DesktopShortcut)) { throw "$($Context): desktop shortcut was not created" }
    } elseif (Test-Path -LiteralPath $Fixture.DesktopShortcut) {
        throw "$($Context): desktop shortcut was created while disabled"
    }
    if ($Start -eq "1") {
        if (-not (Test-Path -LiteralPath $Fixture.StartShortcut)) { throw "$($Context): start shortcut was not created" }
    } elseif (Test-Path -LiteralPath $Fixture.StartShortcut) {
        throw "$($Context): start shortcut was created while disabled"
    }
    if (-not (Test-Path -LiteralPath (Join-Path $Fixture.InstallDir "AtlasNote.exe"))) {
        throw "$($Context): fixture product was not installed"
    }
}

try {
    New-Item -ItemType Directory -Path $runnerRoot -Force | Out-Null

    $production = Get-Content -Raw -Encoding utf8 -LiteralPath $productionSource
    $options = Get-Content -Raw -Encoding utf8 -LiteralPath $optionsSource
    if ($production -notmatch '!define MUI_FINISHPAGE_RUN_NOTCHECKED') {
        throw "Production installer must leave the finish launch checkbox unchecked by default"
    }
    if ($production -notmatch 'Page custom AtlasNoteOptionsPageCreate AtlasNoteOptionsPageLeave') {
        throw "Production installer must expose the optional shortcut page"
    }
    if ($options -notmatch 'explorer\.exe') {
        throw "Production finish launch must delegate through Explorer"
    }

    & $makensis $harnessSource
    if ($LASTEXITCODE -ne 0) { throw "NSIS options harness compilation failed with exit code $LASTEXITCODE" }

    $combos = @(
        @{ Name = "both-on"; Desktop = "1"; Start = "1" },
        @{ Name = "desktop-off"; Desktop = "0"; Start = "1" },
        @{ Name = "start-off"; Desktop = "1"; Start = "0" },
        @{ Name = "both-off"; Desktop = "0"; Start = "0" }
    )
    foreach ($combo in $combos) {
        $fixture = New-Fixture -Name $combo.Name
        try {
            Set-OptionEnvironment -Fixture $fixture -Desktop $combo.Desktop -Start $combo.Start
            Invoke-HarnessInstaller -Silent
            Assert-State -Fixture $fixture -Desktop $combo.Desktop -Start $combo.Start -Context $combo.Name
            Invoke-HarnessInstaller -Silent
            Assert-State -Fixture $fixture -Desktop $combo.Desktop -Start $combo.Start -Context "$($combo.Name) update"
            Write-Output ("PASS: " + $combo.Name + " and update")
        } finally {
            Remove-FixtureSafely -Root $fixture.Root
        }
    }

    $defaultFixture = New-Fixture -Name "silent-default"
    try {
        Set-OptionEnvironment -Fixture $defaultFixture
        Invoke-HarnessInstaller -Silent
        Assert-State -Fixture $defaultFixture -Desktop "1" -Start "1" -Context "silent defaults"
        Write-Output "PASS: silent defaults"
    } finally {
        Remove-FixtureSafely -Root $defaultFixture.Root
    }

    $customFixture = New-Fixture -Name "custom-install" -CustomInstallPath
    try {
        Set-OptionEnvironment -Fixture $customFixture -Desktop "1" -Start "1"
        Invoke-HarnessInstaller -Silent
        Assert-State -Fixture $customFixture -Desktop "1" -Start "1" -Context "custom install"
        if (-not (Test-Path -LiteralPath (Join-Path $customFixture.DefaultInstallDir "untouched.txt"))) {
            throw "custom install: default install directory was touched"
        }
        Write-Output "PASS: custom install path"
    } finally {
        Remove-FixtureSafely -Root $customFixture.Root
    }

    $cancelFixture = New-Fixture -Name "cancel"
    $process = $null
    try {
        Set-OptionEnvironment -Fixture $cancelFixture
        $process = Start-Process -FilePath $harnessBinary -WorkingDirectory $projectRoot -PassThru
        $deadline = [DateTime]::UtcNow.AddSeconds(12)
        do {
            Start-Sleep -Milliseconds 100
            $process.Refresh()
        } while (-not $process.HasExited -and $process.MainWindowHandle -eq 0 -and [DateTime]::UtcNow -lt $deadline)
        if ($process.HasExited) { throw "cancel: installer exited before cancellation" }
        if ($process.MainWindowHandle -eq 0) { throw "cancel: installer window was not found" }
        $process.CloseMainWindow() | Out-Null
        if (-not $process.WaitForExit(10000)) { throw "cancel: installer did not exit after window close" }
        if (Test-Path -LiteralPath (Join-Path $cancelFixture.Root "options.ini")) { throw "cancel: install section ran" }
        if (Test-Path -LiteralPath $cancelFixture.DesktopShortcut) { throw "cancel: desktop shortcut was created" }
        if (Test-Path -LiteralPath $cancelFixture.StartShortcut) { throw "cancel: start shortcut was created" }
        Write-Output "PASS: cancel"
    } finally {
        if ($null -ne $process -and -not $process.HasExited) { $process.Kill() }
        Remove-FixtureSafely -Root $cancelFixture.Root
    }

    $finishFixture = New-Fixture -Name "finish-launch"
    try {
        Set-OptionEnvironment -Fixture $finishFixture -Desktop "0" -Start "0" -FinishLaunch "1" -LaunchMarker $finishFixture.LaunchMarker
        Invoke-HarnessInstaller -Silent
        if (-not (Test-Path -LiteralPath $finishFixture.LaunchMarker)) { throw "finish launch: callback did not run" }
        Assert-State -Fixture $finishFixture -Desktop "0" -Start "0" -Context "finish launch"
        Write-Output "PASS: finish launch callback"
    } finally {
        Remove-FixtureSafely -Root $finishFixture.Root
    }

    Write-Output "All installer options harness tests passed."
} catch {
    Write-Error $_
    exit 1
} finally {
    foreach ($name in $environmentNames) {
        [System.Environment]::SetEnvironmentVariable($name, $savedEnvironment[$name], "Process")
    }
    if (Test-Path -LiteralPath $harnessBinary) {
        Remove-Item -LiteralPath $harnessBinary -Force
    }
    if (Test-Path -LiteralPath $runnerRoot) {
        Remove-FixtureSafely -Root $runnerRoot
    }
}
