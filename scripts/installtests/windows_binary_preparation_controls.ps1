param(
    [Parameter(Mandatory = $true)][string]$InstallerPath,
    [Parameter(Mandatory = $true)][string]$TempDir,
    [string]$Scenario = ''
)

$ErrorActionPreference = 'Stop'

# These independent hash/ACL oracles are native engine prerequisites, not
# optional controls. Keep their identities when a parent engine's environment
# would otherwise prevent the script-backed Get-FileHash from loading.
foreach ($entry in @(
    @('Get-FileHash', 'Microsoft.PowerShell.Utility'),
    @('Get-Acl', 'Microsoft.PowerShell.Security'),
    @('Set-Acl', 'Microsoft.PowerShell.Security')
)) {
    $command = Get-Command -Name $entry[0] -ErrorAction Stop
    if ($command.Source -cne $entry[1]) { throw "Unexpected native prerequisite source: $($entry[0])" }
    Write-Output "NATIVE_PREPARATION_COMMAND $($entry[0]) source=$($command.Source) version=$($command.Version)"
}

# Execute the actual production functions and replacement caller. No network,
# executable launch or real SCM operation is admitted by this local file test.
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $InstallerPath, [ref]$null, [ref]$errors)
if ($errors.Count) { throw "Installer parse failed: $errors" }
foreach ($name in @('Cleanup', 'Get-FileChecksum', 'New-PulsePreparedBinary')) {
    $functions = @($ast.FindAll({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true))
    if ($functions.Count -ne 1) { throw "Expected one production function: $name" }
    Invoke-Expression $functions[0].Extent.Text
}
$installer = [System.IO.File]::ReadAllText($InstallerPath)
$start = $installer.IndexOf('# --- Prepare Verified Binary ---')
$end = $installer.IndexOf('# --- Service Installation ---')
if ($start -lt 0 -or $end -le $start) { throw 'Missing production preparation/replacement caller.' }
$replacement = $installer.Substring($start, $end - $start)

if ($Scenario -ne '') {
    $TempPath = Join-Path $TempDir 'download.exe'
    $InstallDir = Join-Path $TempDir 'installed'
    $DestPath = Join-Path $InstallDir 'pulse-agent.exe'
    $localChecksum = [System.IO.File]::ReadAllText((Join-Path $TempDir 'verified.sha256'))
    $script:TempFiles = @($TempPath)
    $AgentName = 'PulseAgent'
    function Show-Error { param([string]$Message) Write-Output $Message }
    function Remove-PulseService {
        if ($Scenario -eq 'stop-refused') { throw 'Test-only SCM refusal.' }
        [System.IO.File]::WriteAllText((Join-Path $TempDir 'service-removed'), 'test-only SCM double')
    }
    if ($Scenario -eq 'changed-source') {
        [System.IO.File]::AppendAllText($TempPath, 'changed after verification')
    }
    if ($Scenario -eq 'invalid-checksum') { $localChecksum = 'invalid' }
    $locked = $null
    try {
        if ($Scenario -eq 'locked-source') {
            $locked = [System.IO.File]::Open($TempPath, 'Open', 'Read', 'None')
        }
        if ($Scenario -eq 'source-cleanup-locked') {
            $locked = [System.IO.File]::Open($TempPath, 'Open', 'Read', 'Read')
        }
        if ($Scenario -eq 'publication-locked') {
            $locked = [System.IO.File]::Open($DestPath, 'Open', 'Read', 'Read')
        }
        Invoke-Expression $replacement
        Cleanup
    } finally {
        if ($null -ne $locked) { $locked.Dispose() }
    }
    exit 0
}

function Assert-Control {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) { throw "$script:caseName : $Message" }
}

function File-Hash {
    param([string]$Path)
    return (Get-FileHash -LiteralPath $Path -Algorithm SHA256 -ErrorAction Stop).Hash
}

function Invoke-Control {
    param([string]$CaseRoot, [string]$Name)
    $info = [System.Diagnostics.ProcessStartInfo]::new()
    $info.FileName = Join-Path $PSHOME 'powershell.exe'
    $info.Arguments = '-NoLogo -NoProfile -NonInteractive -File "' + $PSCommandPath +
        '" -InstallerPath "' + $InstallerPath + '" -TempDir "' + $CaseRoot + '" -Scenario "' + $Name + '"'
    $info.UseShellExecute = $false
    $info.CreateNoWindow = $true
    $info.RedirectStandardOutput = $true
    $info.RedirectStandardError = $true
    $child = [System.Diagnostics.Process]::Start($info)
    try {
        $stdout = $child.StandardOutput.ReadToEndAsync()
        $stderr = $child.StandardError.ReadToEndAsync()
        if (-not $child.WaitForExit(15000)) {
            $child.Kill()
            $null = $child.WaitForExit(5000)
            throw 'Owned preparation control did not finish.'
        }
        $tasks = [System.Threading.Tasks.Task[]]@($stdout, $stderr)
        if (-not [System.Threading.Tasks.Task]::WaitAll($tasks, 5000)) { throw 'Child output did not close.' }
        return @{ Exit = $child.ExitCode; Output = $stdout.Result + $stderr.Result }
    } finally {
        $child.Dispose()
    }
}

$cases = @('installed-success', 'fresh-success', 'missing-source', 'locked-source', 'source-cleanup-locked',
    'changed-source', 'invalid-checksum', 'destination-denied', 'stop-refused', 'publication-locked')
foreach ($script:caseName in $cases) {
    $root = Join-Path $TempDir $script:caseName
    $installed = Join-Path $root 'installed'
    $state = Join-Path $root 'state'
    $null = [System.IO.Directory]::CreateDirectory($installed)
    $null = [System.IO.Directory]::CreateDirectory($state)
    $source = Join-Path $root 'download.exe'
    $dest = Join-Path $installed 'pulse-agent.exe'
    $untouched = Join-Path $installed '.pulse-agent-not-owned.exe'
    [System.IO.File]::WriteAllBytes($source, [byte[]](0..255))
    [System.IO.File]::WriteAllText((Join-Path $root 'verified.sha256'), (File-Hash $source).ToLowerInvariant())
    [System.IO.File]::WriteAllText($untouched, 'unrelated file: never sweep staging names')
    if ($script:caseName -ne 'fresh-success') { [System.IO.File]::WriteAllText($dest, 'original executable') }
    $before = @{}
    foreach ($name in @('agent-id', 'token', 'connection.env')) {
        $path = Join-Path $state $name
        [System.IO.File]::WriteAllText($path, 'public fixture saved state: ' + $name)
        $before[$path] = File-Hash $path
    }
    $sourceHash = File-Hash $source
    $untouchedHash = File-Hash $untouched
    if ($script:caseName -eq 'missing-source') { [System.IO.File]::Delete($source) }
    $originalAcl = $null
    try {
        if ($script:caseName -eq 'destination-denied') {
            $originalAcl = Get-Acl -LiteralPath $installed
            $acl = Get-Acl -LiteralPath $installed
            $rule = [System.Security.AccessControl.FileSystemAccessRule]::new(
                [System.Security.Principal.WindowsIdentity]::GetCurrent().User,
                [System.Security.AccessControl.FileSystemRights]::CreateFiles,
                [System.Security.AccessControl.AccessControlType]::Deny)
            $acl.AddAccessRule($rule)
            Set-Acl -LiteralPath $installed -AclObject $acl
        }
        $result = Invoke-Control -CaseRoot $root -Name $script:caseName
    } finally {
        if ($null -ne $originalAcl) { Set-Acl -LiteralPath $installed -AclObject $originalAcl }
    }
    $success = $script:caseName -in @('installed-success', 'fresh-success')
    Assert-Control ($result.Exit -eq $(if ($success) { 0 } else { 1 })) "unexpected child exit: $($result.Exit); $($result.Output)"
    $removed = Test-Path -LiteralPath (Join-Path $root 'service-removed')
    Assert-Control ($removed -eq ($success -or $script:caseName -eq 'publication-locked')) 'incorrect service mutation ordering'
    if ($success) {
        Assert-Control ((File-Hash $dest) -ceq $sourceHash) 'published bytes differ from the verified source'
        Assert-Control (-not (Test-Path -LiteralPath $source)) 'original owned download remains after successful preparation'
    } else {
        Assert-Control ([System.IO.File]::ReadAllText($dest) -ceq 'original executable') 'failure changed the original executable'
        if ($script:caseName -notin @('stop-refused', 'publication-locked')) {
            Assert-Control ($result.Output.Contains('Cannot prepare the agent binary')) 'preparation failure was not identified'
        }
    }
    foreach ($path in $before.Keys) { Assert-Control ((File-Hash $path) -ceq $before[$path]) 'saved state changed' }
    Assert-Control ((File-Hash $untouched) -ceq $untouchedHash) 'cleanup erased or changed an unowned staging-looking file'
    $staged = @(Get-ChildItem -LiteralPath $installed -Force | Where-Object { $_.Name -like '.pulse-agent-*.exe' })
    Assert-Control ($staged.Count -eq 1 -and $staged[0].Name -ceq '.pulse-agent-not-owned.exe') 'owned stage was not cleaned or published'
    Write-Output "PASS $script:caseName exit=$($result.Exit) service_removed=$removed saved_state=retained owned_stage=absent"
}

# Two real successful preparations must get distinct exclusive leaves, inherit
# the destination ACL (not the deliberately private source ACL), and release
# both file handles. These are file tests, not Windows service acceptance.
$script:caseName = 'inherited-ACL/unique-stage'
$dir = Join-Path $TempDir 'acl-and-unique'
$null = [System.IO.Directory]::CreateDirectory($dir)
$source = Join-Path $TempDir 'private-download.exe'
[System.IO.File]::WriteAllBytes($source, [byte[]](0..255))
$sourceAcl = Get-Acl -LiteralPath $source
$sourceAcl.SetAccessRuleProtection($true, $false)
$sourceAcl.AddAccessRule([System.Security.AccessControl.FileSystemAccessRule]::new(
    [System.Security.Principal.WindowsIdentity]::GetCurrent().User, 'FullControl', 'Allow'))
Set-Acl -LiteralPath $source -AclObject $sourceAcl
$reference = Join-Path $dir 'inherited-reference'
[System.IO.File]::WriteAllText($reference, 'ACL reference')
$expectedAcl = (Get-Acl -LiteralPath $reference).GetSecurityDescriptorSddlForm('Access')
$script:TempFiles = @()
$checksum = (File-Hash $source).ToLowerInvariant()
$one = New-PulsePreparedBinary -SourcePath $source -DestinationDirectory $dir -ExpectedChecksum $checksum
$two = New-PulsePreparedBinary -SourcePath $source -DestinationDirectory $dir -ExpectedChecksum $checksum.ToUpperInvariant()
Assert-Control ($one -cne $two -and $script:TempFiles.Count -eq 2) 'preparation reused or failed to track an owned leaf'
foreach ($path in @($one, $two)) {
    Assert-Control ((File-Hash $path) -ceq (File-Hash $source)) 'stage bytes differ'
    Assert-Control ((Get-Acl -LiteralPath $path).GetSecurityDescriptorSddlForm('Access') -ceq $expectedAcl) 'stage did not inherit destination access rules'
    $handle = [System.IO.File]::Open($path, 'Open', 'Read', 'None')
    $handle.Dispose()
}
$handle = [System.IO.File]::Open($source, 'Open', 'Read', 'None')
$handle.Dispose()
Cleanup
Assert-Control ((-not (Test-Path -LiteralPath $one)) -and (-not (Test-Path -LiteralPath $two)) -and
    (Test-Path -LiteralPath $source) -and (Test-Path -LiteralPath $reference)) 'cleanup did not respect exact ownership'
Write-Output 'PASS all 10 Windows binary preparation controls and inherited-ACL/unique-stage controls'
