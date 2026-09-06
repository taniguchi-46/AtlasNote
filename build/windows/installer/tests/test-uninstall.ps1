$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$scriptRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = [System.IO.Path]::GetFullPath((Join-Path $scriptRoot "..\..\..\.."))
$harnessSource = Join-Path $projectRoot "build\windows\installer\tests\uninstall-harness.nsi"
$harnessBinary = Join-Path $projectRoot "build\windows\installer\uninstall-harness.exe"
$makensis = "C:\Program Files (x86)\NSIS\makensis.exe"
if (-not (Test-Path -LiteralPath $makensis)) {
    $makensis = (Get-Command makensis.exe -ErrorAction Stop).Source
}

$runnerRoot = Join-Path ([System.IO.Path]::GetTempPath()) ("AtlasNote-uninstall-tests-" + [guid]::NewGuid().ToString("N"))
$registryParentPath = "Software\AtlasNote\InstallerTests"
$trackedEnvironmentNames = @(
    "ATLAS_NOTE_UNINSTALL_FIXTURE",
    "ATLAS_NOTE_UNINSTALL_REGISTRY_KEY",
    "ATLAS_NOTE_UNINSTALL_INSTALL_DIR",
    "ATLAS_NOTE_UNINSTALL_START_SHORTCUT",
    "ATLAS_NOTE_UNINSTALL_DESKTOP_SHORTCUT",
    "ATLAS_NOTE_UNINSTALL_DEFAULT_INSTALL_DIR",
    "ATLAS_NOTE_UNINSTALL_COMPANY_PARENT"
)
$savedEnvironment = @{}
foreach ($name in $trackedEnvironmentNames) {
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

function Set-HarnessEnvironment {
    param(
        [Parameter(Mandatory = $true)]$Fixture,
        [Parameter(Mandatory = $true)][string]$RegistryPath
    )
    $values = @{
        "ATLAS_NOTE_UNINSTALL_FIXTURE" = $Fixture.Root
        "ATLAS_NOTE_UNINSTALL_REGISTRY_KEY" = $RegistryPath
        "ATLAS_NOTE_UNINSTALL_INSTALL_DIR" = $Fixture.InstallDir
        "ATLAS_NOTE_UNINSTALL_START_SHORTCUT" = $Fixture.StartShortcut
        "ATLAS_NOTE_UNINSTALL_DESKTOP_SHORTCUT" = $Fixture.DesktopShortcut
        "ATLAS_NOTE_UNINSTALL_DEFAULT_INSTALL_DIR" = $Fixture.DefaultInstallDir
        "ATLAS_NOTE_UNINSTALL_COMPANY_PARENT" = $Fixture.CompanyParent
    }
    foreach ($entry in $values.GetEnumerator()) {
        [System.Environment]::SetEnvironmentVariable($entry.Key, $entry.Value, "Process")
    }
}

function Invoke-HarnessInstaller {
    $process = Start-Process -FilePath $harnessBinary -ArgumentList @("/S") -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -Wait
    if (-not $process.HasExited) {
        throw "Harness installer did not exit"
    }
    if ($process.ExitCode -ne 0) {
        throw "Harness installer failed with exit code $($process.ExitCode)"
    }
}

function Invoke-HarnessUninstaller {
    param([Parameter(Mandatory = $true)]$Fixture)
    # _?= keeps the uninstaller's error level in the process that PowerShell
    # waits for. NSIS documents this form for a caller that needs the real
    # uninstaller result; the argument must be the final unquoted argument.
    $runnerExecutable = Join-Path $runnerRoot ("uninstaller-runner-" + [guid]::NewGuid().ToString("N") + ".exe")
    Copy-Item -LiteralPath $Fixture.UninstallPath -Destination $runnerExecutable -Force
    try {
        $process = Start-Process -FilePath $runnerExecutable -ArgumentList @("/S", "_?=$($Fixture.InstallDir)") -WorkingDirectory $projectRoot -WindowStyle Hidden -PassThru -Wait
        if (-not $process.HasExited) {
            throw "Harness uninstaller did not exit"
        }
        return $process.ExitCode
    } finally {
        if (Test-Path -LiteralPath $runnerExecutable) {
            Remove-Item -LiteralPath $runnerExecutable -Force
        }
    }
}

function Assert-ExitCode {
    param([int]$Actual, [int]$Expected, [string]$Context)
    if ($Actual -ne $Expected) {
        throw "$($Context): exit code was $Actual, expected $Expected"
    }
}

function Assert-PathExists {
    param([string]$Path, [string]$Context)
    if (-not (Test-Path -LiteralPath $Path)) {
        throw "$($Context): expected path to exist: $Path"
    }
}

function Assert-PathAbsent {
    param([string]$Path, [string]$Context)
    if (Test-Path -LiteralPath $Path) {
        throw "$($Context): expected path to be absent: $Path"
    }
}

function Open-TestRegistryBase {
    return [Microsoft.Win32.RegistryKey]::OpenBaseKey(
        [Microsoft.Win32.RegistryHive]::CurrentUser,
        [Microsoft.Win32.RegistryView]::Registry64
    )
}

function Test-RegistryKey {
    param([string]$Path)
    $base = Open-TestRegistryBase
    try {
        $key = $base.OpenSubKey($Path, $false)
        try {
            return $null -ne $key
        } finally {
            if ($null -ne $key) { $key.Dispose() }
        }
    } finally {
        $base.Dispose()
    }
}

function New-RegistryFixture {
    param([Parameter(Mandatory = $true)]$Fixture)
    $id = [guid]::NewGuid().ToString("N")
    $registryPath = "$registryParentPath\$id"
    $base = Open-TestRegistryBase
    try {
        $key = $base.CreateSubKey($registryPath)
        try {
            $key.SetValue("Publisher", "Atlas Note test", [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("DisplayName", "Atlas Note uninstall harness", [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("DisplayVersion", "0.1.0", [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("DisplayIcon", (Join-Path $Fixture.InstallDir "AtlasNote.exe"), [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("UninstallString", "`"$($Fixture.UninstallPath)`"", [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("QuietUninstallString", "`"$($Fixture.UninstallPath)`" /S", [Microsoft.Win32.RegistryValueKind]::String)
            $key.SetValue("EstimatedSize", [int]1, [Microsoft.Win32.RegistryValueKind]::DWord)
        } finally {
            $key.Dispose()
        }
    } finally {
        $base.Dispose()
    }
    return $registryPath
}

function Remove-RegistryFixture {
    param([Parameter(Mandatory = $true)][string]$Path)
    $base = Open-TestRegistryBase
    try {
        $parent = $base.OpenSubKey($registryParentPath, $true)
        if ($null -ne $parent) {
            try {
                $leaf = Split-Path -Leaf ($Path -replace "\\", [System.IO.Path]::DirectorySeparatorChar)
                $parent.DeleteSubKeyTree($leaf, $false)
            } finally {
                $parent.Dispose()
            }
        }
    } finally {
        $base.Dispose()
    }
}

function Add-RegistryDeleteDeny {
    param([Parameter(Mandatory = $true)][string]$Path)
    $base = Open-TestRegistryBase
    try {
        $key = $base.OpenSubKey($Path, $true)
        if ($null -eq $key) { throw "Registry fixture key is missing: $Path" }
        try {
            $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
            $rule = [System.Security.AccessControl.RegistryAccessRule]::new(
                $sid,
                [System.Security.AccessControl.RegistryRights]::Delete,
                [System.Security.AccessControl.InheritanceFlags]::None,
                [System.Security.AccessControl.PropagationFlags]::None,
                [System.Security.AccessControl.AccessControlType]::Deny
            )
            $acl = $key.GetAccessControl()
            $acl.AddAccessRule($rule)
            $key.SetAccessControl($acl)
            return $rule
        } finally {
            $key.Dispose()
        }
    } finally {
        $base.Dispose()
    }
}

function Remove-RegistryDeleteDeny {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)]$Rule)
    $base = Open-TestRegistryBase
    try {
        $key = $base.OpenSubKey($Path, $true)
        if ($null -ne $key) {
            try {
                $acl = $key.GetAccessControl()
                $acl.RemoveAccessRuleSpecific($Rule)
                $key.SetAccessControl($acl)
            } finally {
                $key.Dispose()
            }
        }
    } finally {
        $base.Dispose()
    }
}

function Add-DirectoryCreateDeny {
    param([Parameter(Mandatory = $true)][string]$Path)
    $sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
    $rights = [System.Security.AccessControl.FileSystemRights]::CreateFiles -bor [System.Security.AccessControl.FileSystemRights]::CreateDirectories
    $rule = [System.Security.AccessControl.FileSystemAccessRule]::new(
        $sid, $rights,
        [System.Security.AccessControl.InheritanceFlags]::None,
        [System.Security.AccessControl.PropagationFlags]::None,
        [System.Security.AccessControl.AccessControlType]::Deny
    )
    $acl = Get-Acl -LiteralPath $Path
    $acl.AddAccessRule($rule)
    Set-Acl -LiteralPath $Path -AclObject $acl
    return $rule
}

function Remove-DirectoryCreateDeny {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)]$Rule)
    $acl = Get-Acl -LiteralPath $Path
    $acl.RemoveAccessRuleSpecific($Rule)
    Set-Acl -LiteralPath $Path -AclObject $acl
}

function New-Fixture {
    param([switch]$CustomInstallPath)
    $root = Join-Path $runnerRoot ([guid]::NewGuid().ToString("N"))
    $installDir = if ($CustomInstallPath) { Join-Path $root "custom-install" } else { Join-Path $root "install" }
    $defaultInstallDir = if ($CustomInstallPath) { Join-Path $root "default-install" } else { $installDir }
    $fixture = [pscustomobject]@{
        Root = $root
        InstallDir = $installDir
        UninstallPath = Join-Path $installDir "uninstall.exe"
        ProductPath = Join-Path $installDir "AtlasNote.exe"
        StartShortcut = Join-Path $root "shortcuts\start.lnk"
        DesktopShortcut = Join-Path $root "shortcuts\desktop.lnk"
        DefaultInstallDir = $defaultInstallDir
        CompanyParent = Join-Path $root "company-parent"
        RegistryPath = $null
    }
    New-Item -ItemType Directory -Path $root -Force | Out-Null
    New-Item -ItemType Directory -Path $fixture.InstallDir -Force | Out-Null
    New-Item -ItemType Directory -Path (Split-Path -Parent $fixture.StartShortcut) -Force | Out-Null
    New-Item -ItemType Directory -Path $fixture.CompanyParent -Force | Out-Null
    if ($CustomInstallPath) {
        New-Item -ItemType Directory -Path $fixture.DefaultInstallDir -Force | Out-Null
    }
    Set-HarnessEnvironment -Fixture $fixture -RegistryPath "pending"
    Invoke-HarnessInstaller
    [System.IO.File]::WriteAllText($fixture.ProductPath, "Atlas Note product fixture")
    [System.IO.File]::WriteAllText($fixture.StartShortcut, "start shortcut")
    [System.IO.File]::WriteAllText($fixture.DesktopShortcut, "desktop shortcut")
    $fixture.RegistryPath = New-RegistryFixture -Fixture $fixture
    Set-HarnessEnvironment -Fixture $fixture -RegistryPath $fixture.RegistryPath
    return $fixture
}

function Remove-Fixture {
    param([Parameter(Mandatory = $true)]$Fixture)
    if ($null -ne $Fixture.RegistryPath) {
        Remove-RegistryFixture -Path $Fixture.RegistryPath
    }
    Remove-FixtureSafely -Root $Fixture.Root
}

function Invoke-UninstallWithReadLock {
    param([Parameter(Mandatory = $true)][string]$Path, [Parameter(Mandatory = $true)]$Fixture)
    $stream = [System.IO.File]::Open(
        $Path,
        [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::Read,
        [System.IO.FileShare]::Read
    )
    try {
        return Invoke-HarnessUninstaller -Fixture $Fixture
    } finally {
        $stream.Dispose()
    }
}

function Test-ApplicationLock {
    $fixture = New-Fixture
    try {
        $exitCode = Invoke-UninstallWithReadLock -Path $fixture.ProductPath -Fixture $fixture
        Assert-ExitCode $exitCode 1 "locked product executable"
        Assert-PathExists $fixture.ProductPath "locked product executable"
        Assert-PathExists $fixture.UninstallPath "locked product executable"
        Assert-PathExists $fixture.StartShortcut "locked product executable"
        Assert-PathExists $fixture.DesktopShortcut "locked product executable"
        if (-not (Test-RegistryKey $fixture.RegistryPath)) { throw "locked product executable: registry entry was changed" }
        Assert-PathAbsent (Join-Path $fixture.InstallDir "AtlasNote-uninstall-backup.exe") "locked product executable"
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "product unlock retry"
        Assert-PathAbsent $fixture.ProductPath "product unlock retry"
        Assert-PathAbsent $fixture.UninstallPath "product unlock retry"
        Assert-PathAbsent $fixture.StartShortcut "product unlock retry"
        Assert-PathAbsent $fixture.DesktopShortcut "product unlock retry"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "product unlock retry: registry entry remains" }
    } finally {
        Remove-Fixture -Fixture $fixture
    }
}

function Test-UninstallerLock {
    $fixture = New-Fixture
    try {
        $exitCode = Invoke-UninstallWithReadLock -Path $fixture.UninstallPath -Fixture $fixture
        Assert-ExitCode $exitCode 1 "locked uninstall executable"
        Assert-PathAbsent $fixture.ProductPath "locked uninstall executable"
        Assert-PathExists $fixture.UninstallPath "locked uninstall executable"
        Assert-PathAbsent $fixture.StartShortcut "locked uninstall executable"
        Assert-PathAbsent $fixture.DesktopShortcut "locked uninstall executable"
        if (-not (Test-RegistryKey $fixture.RegistryPath)) { throw "locked uninstall executable: registry entry was changed" }
        Assert-PathAbsent (Join-Path $fixture.InstallDir "AtlasNote-uninstall-backup.exe") "locked uninstall executable"
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "uninstall unlock retry"
        Assert-PathAbsent $fixture.UninstallPath "uninstall unlock retry"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "uninstall unlock retry: registry entry remains" }
    } finally {
        Remove-Fixture -Fixture $fixture
    }
}

function Test-UserContentIsPreserved {
    $fixture = New-Fixture
    try {
        $keepFile = Join-Path $fixture.InstallDir "keep.txt"
        $keepChild = Join-Path $fixture.InstallDir "user-folder\nested.txt"
        $oldAlias = Join-Path $fixture.InstallDir "AtlasNote-uninstall-backup.exe"
        $companyKeep = Join-Path $fixture.CompanyParent "company-keep.txt"
        New-Item -ItemType Directory -Path (Split-Path -Parent $keepChild) -Force | Out-Null
        [System.IO.File]::WriteAllText($keepFile, "keep")
        [System.IO.File]::WriteAllText($keepChild, "nested keep")
        [System.IO.File]::WriteAllText($oldAlias, "user-owned old alias")
        [System.IO.File]::WriteAllText($companyKeep, "company keep")
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "user content cleanup"
        Assert-PathExists $keepFile "user content cleanup"
        Assert-PathExists $keepChild "user content cleanup"
        Assert-PathExists $oldAlias "user content cleanup"
        Assert-PathExists $companyKeep "user content cleanup"
        Assert-PathAbsent $fixture.ProductPath "user content cleanup"
        Assert-PathAbsent $fixture.UninstallPath "user content cleanup"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "user content cleanup: registry entry remains" }
    } finally {
        Remove-Fixture -Fixture $fixture
    }
}

function Test-CustomInstallDoesNotTouchDefaultCompanyParent {
    $fixture = New-Fixture -CustomInstallPath
    try {
        $defaultSentinel = Join-Path $fixture.DefaultInstallDir "default-sentinel.txt"
        $companySentinel = Join-Path $fixture.CompanyParent "company-sentinel.txt"
        [System.IO.File]::WriteAllText($defaultSentinel, "default untouched")
        [System.IO.File]::WriteAllText($companySentinel, "company untouched")
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "custom install cleanup"
        Assert-PathAbsent $fixture.InstallDir "custom install cleanup"
        Assert-PathExists $defaultSentinel "custom install cleanup"
        Assert-PathExists $companySentinel "custom install cleanup"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "custom install cleanup: registry entry remains" }
    } finally {
        Remove-Fixture -Fixture $fixture
    }
}

function Test-RegistryDeleteFailureRestoresRetryState {
    $fixture = New-Fixture
    $denyRule = $null
    try {
        $denyRule = Add-RegistryDeleteDeny -Path $fixture.RegistryPath
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 1 "registry delete denial"
        Assert-PathAbsent $fixture.ProductPath "registry delete denial"
        Assert-PathExists $fixture.UninstallPath "registry delete denial"
        if (-not (Test-RegistryKey $fixture.RegistryPath)) { throw "registry delete denial: registry entry was not retained" }
        Remove-RegistryDeleteDeny -Path $fixture.RegistryPath -Rule $denyRule
        $denyRule = $null
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "registry delete retry"
        Assert-PathAbsent $fixture.UninstallPath "registry delete retry"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "registry delete retry: registry entry remains" }
    } finally {
        if ($null -ne $denyRule) { Remove-RegistryDeleteDeny -Path $fixture.RegistryPath -Rule $denyRule }
        Remove-Fixture -Fixture $fixture
    }
}

function Test-RestoreFailureIsReportedAndReinstallRecovers {
    $fixture = New-Fixture
    $registryDeny = $null
    $directoryDeny = $null
    try {
        $registryDeny = Add-RegistryDeleteDeny -Path $fixture.RegistryPath
        $directoryDeny = Add-DirectoryCreateDeny -Path $fixture.InstallDir
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 1 "uninstaller restore failure"
        Assert-PathAbsent $fixture.ProductPath "uninstaller restore failure"
        Assert-PathAbsent $fixture.UninstallPath "uninstaller restore failure"
        if (-not (Test-RegistryKey $fixture.RegistryPath)) { throw "uninstaller restore failure: registry entry disappeared" }
        Remove-DirectoryCreateDeny -Path $fixture.InstallDir -Rule $directoryDeny
        $directoryDeny = $null
        Remove-RegistryDeleteDeny -Path $fixture.RegistryPath -Rule $registryDeny
        $registryDeny = $null
        Invoke-HarnessInstaller
        Assert-PathExists $fixture.UninstallPath "reinstall recovery"
        $exitCode = Invoke-HarnessUninstaller -Fixture $fixture
        Assert-ExitCode $exitCode 0 "reinstall recovery"
        if (Test-RegistryKey $fixture.RegistryPath) { throw "reinstall recovery: registry entry remains" }
    } finally {
        if ($null -ne $directoryDeny) { Remove-DirectoryCreateDeny -Path $fixture.InstallDir -Rule $directoryDeny }
        if ($null -ne $registryDeny) { Remove-RegistryDeleteDeny -Path $fixture.RegistryPath -Rule $registryDeny }
        Remove-Fixture -Fixture $fixture
    }
}

try {
    New-Item -ItemType Directory -Path $runnerRoot -Force | Out-Null
    & $makensis $harnessSource
    if ($LASTEXITCODE -ne 0) { throw "NSIS harness compilation failed with exit code $LASTEXITCODE" }

    $tests = @(
        @{ Name = "application lock"; Action = { Test-ApplicationLock } },
        @{ Name = "uninstaller lock"; Action = { Test-UninstallerLock } },
        @{ Name = "user content"; Action = { Test-UserContentIsPreserved } },
        @{ Name = "custom install"; Action = { Test-CustomInstallDoesNotTouchDefaultCompanyParent } },
        @{ Name = "registry delete denial"; Action = { Test-RegistryDeleteFailureRestoresRetryState } },
        @{ Name = "restore failure"; Action = { Test-RestoreFailureIsReportedAndReinstallRecovers } }
    )
    foreach ($test in $tests) {
        & $test.Action
        Write-Output ("PASS: " + $test.Name)
    }
    Write-Output "All uninstall harness tests passed."
} catch {
    Write-Error $_
    exit 1
} finally {
    foreach ($name in $trackedEnvironmentNames) {
        $value = $savedEnvironment[$name]
        [System.Environment]::SetEnvironmentVariable($name, $value, "Process")
    }
    if (Test-Path -LiteralPath $harnessBinary) {
        Remove-Item -LiteralPath $harnessBinary -Force
    }
    if (Test-Path -LiteralPath $runnerRoot) {
        Remove-FixtureSafely -Root $runnerRoot
    }
}
