#requires -Version 7.0

[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Leaf })]
    [string]$TaskContractPath,

    [Parameter(Mandatory)]
    [ValidateScript({ Test-Path -LiteralPath $_ -PathType Container })]
    [string]$WorkingDirectory,

    [ValidatePattern('^[0-9]+[a-zA-Z0-9.]+$')]
    [string]$PrintTimeout = '5m0s',

    [string]$LogDirectory,

    [switch]$DryRun
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Get-AgyExecutable {
    $command = Get-Command agy -CommandType Application -ErrorAction SilentlyContinue |
        Select-Object -First 1

    if ($null -eq $command) {
        throw 'Antigravity CLI (agy) was not found on PATH. Install or configure agy before delegating.'
    }

    if (-not [string]::IsNullOrWhiteSpace($command.Path)) {
        return $command.Path
    }

    return $command.Source
}

function Get-ResolvedPath {
    param(
        [Parameter(Mandatory)]
        [string]$Path
    )

    return (Resolve-Path -LiteralPath $Path -ErrorAction Stop).Path
}

$resolvedTaskContractPath = Get-ResolvedPath -Path $TaskContractPath
$resolvedWorkingDirectory = Get-ResolvedPath -Path $WorkingDirectory
$agyExecutable = Get-AgyExecutable
$utf8NoBom = [System.Text.UTF8Encoding]::new($false)
$contract = [System.IO.File]::ReadAllText($resolvedTaskContractPath, $utf8NoBom)

if ([string]::IsNullOrWhiteSpace($contract)) {
    throw 'TaskContractPath points to an empty file. Supply a complete bounded Task Contract.'
}

$prompt = @"
You are the implementation delegate. Treat the supplied Task Contract as authoritative.
Before editing, inspect only the related code and tests.
Change only Allowed scope. Preserve Protected pre-existing changes.
Follow repository instructions, existing architecture, naming, error handling, and code style.
Do not add dependencies or perform work declared Out of scope.
Do not commit, push, create or delete branches or worktrees, reset, checkout, rebase, clean, force, or discard changes.
Run only the supplied validation commands when safe.
At the end, report changed files, implementation details, each command and result, and unresolved items.

Task Contract:
----------------
$contract
"@

$arguments = [System.Collections.Generic.List[string]]::new()
[void]$arguments.Add('--print')
[void]$arguments.Add('--sandbox')
[void]$arguments.Add('--print-timeout')
[void]$arguments.Add($PrintTimeout)
[void]$arguments.Add($prompt)

if ([string]::IsNullOrWhiteSpace($LogDirectory)) {
    $LogDirectory = Join-Path ([System.IO.Path]::GetTempPath()) 'antigravity-implement'
}

$preview = [ordered]@{
    Status = if ($DryRun) { 'DryRun' } else { 'Ready' }
    Executable = $agyExecutable
    WorkingDirectory = $resolvedWorkingDirectory
    TaskContractPath = $resolvedTaskContractPath
    Arguments = @('--print', '--sandbox', '--print-timeout', $PrintTimeout, '<task-contract-prompt-redacted>')
    LogDirectory = $LogDirectory
}

if ($DryRun) {
    [pscustomobject]$preview | ConvertTo-Json -Compress
    exit 0
}

New-Item -ItemType Directory -Path $LogDirectory -Force | Out-Null
$resolvedLogDirectory = Get-ResolvedPath -Path $LogDirectory
$runDirectory = Join-Path $resolvedLogDirectory ('run-' + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $runDirectory -ErrorAction Stop | Out-Null

$stdoutPath = Join-Path $runDirectory 'stdout.log'
$stderrPath = Join-Path $runDirectory 'stderr.log'

$startInfo = [System.Diagnostics.ProcessStartInfo]::new()
$startInfo.FileName = $agyExecutable
$startInfo.WorkingDirectory = $resolvedWorkingDirectory
$startInfo.UseShellExecute = $false
$startInfo.CreateNoWindow = $true
$startInfo.RedirectStandardOutput = $true
$startInfo.RedirectStandardError = $true

foreach ($argument in $arguments) {
    [void]$startInfo.ArgumentList.Add($argument)
}

$process = [System.Diagnostics.Process]::new()
$process.StartInfo = $startInfo

if (-not $process.Start()) {
    throw 'Failed to start Antigravity CLI.'
}

$stdoutTask = $process.StandardOutput.ReadToEndAsync()
$stderrTask = $process.StandardError.ReadToEndAsync()
$process.WaitForExit()

$stdout = $stdoutTask.GetAwaiter().GetResult()
$stderr = $stderrTask.GetAwaiter().GetResult()
[System.IO.File]::WriteAllText($stdoutPath, $stdout, $utf8NoBom)
[System.IO.File]::WriteAllText($stderrPath, $stderr, $utf8NoBom)

[pscustomobject]@{
    Status = if ($process.ExitCode -eq 0) { 'Success' } else { 'Failed' }
    ExitCode = $process.ExitCode
    WorkingDirectory = $resolvedWorkingDirectory
    StdoutPath = $stdoutPath
    StderrPath = $stderrPath
    RunDirectory = $runDirectory
} | ConvertTo-Json -Compress

exit $process.ExitCode
