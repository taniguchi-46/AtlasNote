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

    [string]$AgyExecutable,

    [switch]$Interactive,

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

function Get-HeadlessPermissionBlock {
    param(
        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Stdout,

        [Parameter(Mandatory)]
        [AllowEmptyString()]
        [string]$Stderr
    )

    $combinedOutput = "$Stdout`n$Stderr"
    $match = [regex]::Match(
        $combinedOutput,
        '(?im)jetski:\s+no output produced.*?required the\s+"(?<permission>[^"]+)"\s+permission that headless mode cannot prompt for.*?auto-denied'
    )

    if ($match.Success) {
        $permission = $match.Groups['permission'].Value

        if ($permission -match '^[A-Za-z0-9_-]{1,64}$') {
            return $permission
        }

        return 'unknown'
    }

    return $null
}

$resolvedTaskContractPath = Get-ResolvedPath -Path $TaskContractPath
$resolvedWorkingDirectory = Get-ResolvedPath -Path $WorkingDirectory
if ([string]::IsNullOrWhiteSpace($AgyExecutable)) {
    $agyExecutable = Get-AgyExecutable
} else {
    if (-not (Test-Path -LiteralPath $AgyExecutable -PathType Leaf)) {
        throw 'AgyExecutable must point to an existing executable file.'
    }

    $agyExecutable = Get-ResolvedPath -Path $AgyExecutable
}
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

$headlessArguments = [System.Collections.Generic.List[string]]::new()
[void]$headlessArguments.Add('--sandbox')
[void]$headlessArguments.Add('--print-timeout')
[void]$headlessArguments.Add($PrintTimeout)
[void]$headlessArguments.Add('--print')
[void]$headlessArguments.Add($prompt)

$interactiveHandoff = "Read and follow the Task Contract at `"$resolvedTaskContractPath`". Treat it as authoritative. Before editing, inspect only related code and tests; change only its Allowed scope; run only its validation commands; do not commit, push, or bypass permissions."
$interactiveArguments = @('--sandbox', '--prompt-interactive', $interactiveHandoff)

if ([string]::IsNullOrWhiteSpace($LogDirectory)) {
    $LogDirectory = Join-Path ([System.IO.Path]::GetTempPath()) 'antigravity-implement'
}

$preview = [ordered]@{
    Status = if ($DryRun) { 'DryRun' } else { 'Ready' }
    Mode = if ($Interactive) { 'Interactive' } else { 'Headless' }
    Executable = $agyExecutable
    WorkingDirectory = $resolvedWorkingDirectory
    TaskContractPath = $resolvedTaskContractPath
    Arguments = if ($Interactive) {
        @('--sandbox', '--prompt-interactive', '<task-contract-handoff-supplied-as-initial-prompt>')
    } else {
        @('--sandbox', '--print-timeout', $PrintTimeout, '--print', '<task-contract-prompt-redacted>')
    }
    LogDirectory = $LogDirectory
}

if ($DryRun) {
    [pscustomobject]$preview | ConvertTo-Json -Compress
    exit 0
}

if ($Interactive) {
    Write-Host 'Starting Antigravity in interactive approval mode with the bounded task-contract handoff.'
    Write-Host 'Review and approve or deny each requested tool operation in Antigravity.'

    $interactiveExitCode = 1
    Push-Location -LiteralPath $resolvedWorkingDirectory

    try {
        & $agyExecutable @interactiveArguments
        $interactiveExitCode = $LASTEXITCODE
    } finally {
        Pop-Location
    }

    [pscustomobject]@{
        Status = if ($interactiveExitCode -eq 0) { 'InteractiveCompleted' } else { 'InteractiveFailed' }
        Mode = 'Interactive'
        ExitCode = $interactiveExitCode
        ProcessExitCode = $interactiveExitCode
        BlockedPermission = $null
        WorkingDirectory = $resolvedWorkingDirectory
        TaskContractPath = $resolvedTaskContractPath
    } | ConvertTo-Json -Compress

    exit $interactiveExitCode
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

foreach ($argument in $headlessArguments) {
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

$blockedPermission = Get-HeadlessPermissionBlock -Stdout $stdout -Stderr $stderr
$processExitCode = $process.ExitCode
$wrapperExitCode = $processExitCode

if (-not [string]::IsNullOrWhiteSpace($blockedPermission)) {
    $wrapperExitCode = 3
}

[pscustomobject]@{
    Status = if (-not [string]::IsNullOrWhiteSpace($blockedPermission)) {
        'Blocked'
    } elseif ($processExitCode -eq 0) {
        'Success'
    } else {
        'Failed'
    }
    ExitCode = $wrapperExitCode
    ProcessExitCode = $processExitCode
    BlockedPermission = $blockedPermission
    WorkingDirectory = $resolvedWorkingDirectory
    StdoutPath = $stdoutPath
    StderrPath = $stderrPath
    RunDirectory = $runDirectory
} | ConvertTo-Json -Compress

exit $wrapperExitCode
