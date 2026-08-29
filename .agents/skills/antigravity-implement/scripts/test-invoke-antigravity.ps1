#requires -Version 7.0

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-Equal {
    param(
        [Parameter(Mandatory)]
        $Expected,

        [Parameter(Mandatory)]
        $Actual,

        [Parameter(Mandatory)]
        [string]$Name
    )

    if ($Expected -ne $Actual) {
        throw "${Name}: expected '$Expected', got '$Actual'."
    }
}

function Assert-True {
    param(
        [Parameter(Mandatory)]
        [bool]$Condition,

        [Parameter(Mandatory)]
        [string]$Name
    )

    if (-not $Condition) {
        throw "${Name}: expected true."
    }
}

$testRoot = Join-Path ([System.IO.Path]::GetTempPath()) ('antigravity-wrapper-test-' + [Guid]::NewGuid().ToString('N'))
$previousMode = [System.Environment]::GetEnvironmentVariable('ATLASNOTE_FAKE_AGY_MODE', 'Process')

try {
    $wrapperPath = Join-Path $PSScriptRoot 'invoke-antigravity.ps1'
    $pwshExecutable = (Get-Command pwsh -CommandType Application -ErrorAction Stop | Select-Object -First 1).Path
    $fakeAgyPath = Join-Path $testRoot 'fake-agy.cmd'
    $contractPath = Join-Path $testRoot 'contract.txt'
    $workingDirectory = Join-Path $testRoot 'workspace'

    New-Item -ItemType Directory -Path $testRoot | Out-Null
    New-Item -ItemType Directory -Path $workingDirectory | Out-Null
    [System.IO.File]::WriteAllText($contractPath, "Objective:`nwrapper regression test`n", [System.Text.UTF8Encoding]::new($false))
    [System.IO.File]::WriteAllText($fakeAgyPath, @'
@echo off
if /I "%ATLASNOTE_FAKE_AGY_MODE%"=="permission" (
  echo jetski: no output produced - a tool required the "command" permission that headless mode cannot prompt for, so it was auto-denied.
  exit /b 0
)
if /I "%ATLASNOTE_FAKE_AGY_MODE%"=="interactive" (
  echo fake interactive arguments: %*
  exit /b 0
)
echo delegate completed
exit /b 0
'@, [System.Text.Encoding]::ASCII)

    function Invoke-WrapperCase {
        param(
            [Parameter(Mandatory)]
            [string]$Mode,

            [switch]$Interactive
        )

        $env:ATLASNOTE_FAKE_AGY_MODE = $Mode
        $logDirectory = Join-Path $testRoot ("logs-$Mode")
        $wrapperArguments = @(
            '-NoProfile',
            '-File', $wrapperPath,
            '-TaskContractPath', $contractPath,
            '-WorkingDirectory', $workingDirectory,
            '-LogDirectory', $logDirectory,
            '-AgyExecutable', $fakeAgyPath
        )

        if ($Interactive) {
            $wrapperArguments += '-Interactive'
        }

        $output = @(& $pwshExecutable @wrapperArguments 2>&1)
        $exitCode = $LASTEXITCODE
        $jsonLine = @($output | ForEach-Object { $_.ToString() } | Where-Object { $_ -match '^\{' } | Select-Object -Last 1)

        if ($jsonLine.Count -ne 1) {
            throw "Wrapper did not return exactly one JSON summary for mode '$Mode': $($output -join [Environment]::NewLine)"
        }

        return [pscustomobject]@{
            ExitCode = $exitCode
            Result = $jsonLine[0] | ConvertFrom-Json
            Output = $output | ForEach-Object { $_.ToString() }
        }
    }

    $permissionCase = Invoke-WrapperCase -Mode 'permission'
    Assert-Equal -Expected 3 -Actual $permissionCase.ExitCode -Name 'permission-denied wrapper exit code'
    Assert-Equal -Expected 'Blocked' -Actual $permissionCase.Result.Status -Name 'permission-denied status'
    Assert-Equal -Expected 3 -Actual $permissionCase.Result.ExitCode -Name 'permission-denied JSON exit code'
    Assert-Equal -Expected 0 -Actual $permissionCase.Result.ProcessExitCode -Name 'permission-denied process exit code'
    Assert-Equal -Expected 'command' -Actual $permissionCase.Result.BlockedPermission -Name 'blocked permission'
    Assert-True -Condition (Test-Path -LiteralPath $permissionCase.Result.StdoutPath) -Name 'permission-denied stdout log'
    Assert-True -Condition (Test-Path -LiteralPath $permissionCase.Result.StderrPath) -Name 'permission-denied stderr log'

    $successCase = Invoke-WrapperCase -Mode 'success'
    Assert-Equal -Expected 0 -Actual $successCase.ExitCode -Name 'success wrapper exit code'
    Assert-Equal -Expected 'Success' -Actual $successCase.Result.Status -Name 'success status'
    Assert-Equal -Expected 0 -Actual $successCase.Result.ExitCode -Name 'success JSON exit code'
    Assert-Equal -Expected 0 -Actual $successCase.Result.ProcessExitCode -Name 'success process exit code'
    if ($null -ne $successCase.Result.BlockedPermission) {
        throw "success blocked permission: expected null, got '$($successCase.Result.BlockedPermission)'."
    }

    $interactiveCase = Invoke-WrapperCase -Mode 'interactive' -Interactive
    Assert-Equal -Expected 0 -Actual $interactiveCase.ExitCode -Name 'interactive wrapper exit code'
    Assert-Equal -Expected 'InteractiveCompleted' -Actual $interactiveCase.Result.Status -Name 'interactive status'
    Assert-Equal -Expected 'Interactive' -Actual $interactiveCase.Result.Mode -Name 'interactive mode'
    Assert-Equal -Expected 0 -Actual $interactiveCase.Result.ExitCode -Name 'interactive JSON exit code'
    Assert-Equal -Expected 0 -Actual $interactiveCase.Result.ProcessExitCode -Name 'interactive process exit code'
    Assert-True -Condition (($interactiveCase.Output -join [Environment]::NewLine) -match 'fake interactive arguments: --sandbox --prompt-interactive') -Name 'interactive CLI arguments'
    Assert-True -Condition (($interactiveCase.Output -join [Environment]::NewLine) -match 'Read and follow the Task Contract at') -Name 'interactive initial handoff prompt'
    Assert-True -Condition (($interactiveCase.Output -join [Environment]::NewLine) -notmatch '(?<!interactive )--print') -Name 'interactive excludes print mode'

    'Antigravity wrapper tests passed.'
} finally {
    [System.Environment]::SetEnvironmentVariable('ATLASNOTE_FAKE_AGY_MODE', $previousMode, 'Process')

    if (Test-Path -LiteralPath $testRoot) {
        Remove-Item -LiteralPath $testRoot -Recurse
    }
}
