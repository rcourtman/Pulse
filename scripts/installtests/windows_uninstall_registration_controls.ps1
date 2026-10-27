param(
    [Parameter(Mandatory = $true)][string]$InstallerPath,
    [Parameter(Mandatory = $true)][string]$Scenario
)

$ErrorActionPreference = 'Stop'
$AgentName = 'PulseAgent'
$BinaryName = 'pulse-agent.exe'
$InstallDir = 'C:\synthetic-install'
$StateDir = 'C:\synthetic-state'
$ConnectionStatePath = "$StateDir\connection.env"
$TokenFilePath = "$StateDir\token"
$Uninstall = $true
$Url = ''
$Token = ''
$TokenFile = ''
$AgentId = ''
$Insecure = $false
$CACertPath = ''
$ServerFingerprint = ''
$script:secret = 'synthetic-private-token-never-print'
$script:hasState = $Scenario -ne 'repeated'
$script:state = @{
    PULSE_URL = 'https://same-instance.invalid/'
    PULSE_TOKEN_FILE = $TokenFilePath
    PULSE_AGENT_ID = 'exact-agent:42'
    PULSE_SERVER_FINGERPRINT = 'synthetic-saved-fingerprint'
}
if ($Scenario -eq 'repeated') { $script:state.Clear() }
if ($Scenario -eq 'missing-url') { $script:state.PULSE_URL = '' }
if ($Scenario -in @('agent-id-file', 'missing-id')) { $script:state.PULSE_AGENT_ID = '' }
if ($Scenario -eq 'optional-auth') { $script:state.PULSE_TOKEN_FILE = '' }
if ($Scenario -eq 'legacy-inline-token') {
    $script:state.PULSE_TOKEN_FILE = ''
    $script:state.PULSE_TOKEN = $script:secret
}
if ($Scenario -eq 'explicit-token-file') { $TokenFile = 'C:\explicit-private-token' }
if ($Scenario -eq 'explicit-identity') {
    $AgentId = 'explicit-agent:42'
    $Token = $script:secret
    $Url = 'https://same-instance.invalid/'
}
if ($Scenario -eq 'saved-ca' -or $Scenario -eq 'invalid-ca') { $script:state.PULSE_CACERT = 'C:\synthetic-ca' }
if ($Scenario -eq 'saved-insecure') { $script:state.PULSE_INSECURE_SKIP_VERIFY = 'true' }

function Event([string]$Name) { [Console]::WriteLine("EVENT $Name") }
function Remove-PulseService {
    Event 'service'
    if ($Scenario -eq 'service-fails') { throw 'controlled service failure' }
}
function Show-Error([string]$Message) { [Console]::WriteLine("ERROR $Message") }
function Get-ConnectionStateValue([string]$Key) { return [string]$script:state[$Key] }
function Test-Path {
    param([string]$Path, [string]$LiteralPath)
    $value = $Path
    if ($LiteralPath) { $value = $LiteralPath }
    if ($value -eq "$StateDir\agent-id") { return $Scenario -eq 'agent-id-file' }
    return $script:hasState
}
function Get-Content {
    [CmdletBinding()]
    param([string]$LiteralPath, [switch]$Raw)
    if ($LiteralPath -eq "$StateDir\agent-id") { return "exact-agent:42`r`n" }
    if ($Scenario -eq 'unreadable-token') { throw $script:secret }
    if ($Scenario -eq 'empty-token') { return ' ' }
    $expectedPath = $TokenFilePath
    if ($Scenario -eq 'explicit-token-file') { $expectedPath = 'C:\explicit-private-token' }
    if ($LiteralPath -cne $expectedPath) { throw 'wrong credential path' }
    Event 'credential-read'
    return $script:secret
}
function Load-CustomCaCertificate([string]$Path) {
    Event 'ca-load'
    if ($Scenario -eq 'invalid-ca') { throw $script:secret }
    $certificate = New-Object PSObject
    $certificate | Add-Member -MemberType ScriptMethod -Name Dispose -Value { Event 'ca-dispose' }
    return $certificate
}
function Invoke-WithOptionalInsecureTls {
    param([bool]$AllowInsecure, $CustomCaCertificate, [scriptblock]$Action)
    if ($AllowInsecure -ne ($Scenario -eq 'saved-insecure')) { throw 'changed TLS verification posture' }
    if ($ServerFingerprint -cne 'synthetic-saved-fingerprint') { throw 'lost saved fingerprint' }
    if ($Scenario -eq 'saved-ca' -and $null -eq $CustomCaCertificate) { throw 'lost saved CA' }
    if ($Scenario -eq 'tls-fails') { throw $script:secret }
    & $Action
}
function Invoke-RestMethod {
    [CmdletBinding()]
    param([string]$Uri, [string]$Method, [string]$Body, [string]$ContentType, [int]$TimeoutSec, $Headers)
    Event 'request'
    if ($Uri -cne 'https://same-instance.invalid/api/agents/agent/uninstall' -or
        $Method -cne 'Post' -or $ContentType -cne 'application/json' -or $TimeoutSec -ne 5 -or
        $ErrorActionPreference -ne 'Stop') { throw 'wrong request boundary' }
    $id = 'exact-agent:42'
    if ($Scenario -eq 'explicit-identity') { $id = 'explicit-agent:42' }
    $parsed = $Body | ConvertFrom-Json
    if ($parsed.agentId -cne $id -or @($parsed.PSObject.Properties).Count -ne 1) { throw 'wrong request identity' }
    if ($Scenario -eq 'optional-auth') {
        if ($null -ne $Headers) { throw 'optional authentication gained a header' }
    } elseif ($Headers['X-API-Token'] -cne $script:secret) { throw 'wrong saved credential' }
    if ($Scenario -in @('http-401', 'http-403', 'http-500', 'timeout')) { throw $script:secret }
    switch ($Scenario) {
        'wrong-id' { return [pscustomobject]@{ success = $true; agentId = 'other-agent:42' } }
        'wrong-case-id' { return [pscustomobject]@{ success = $true; agentId = 'EXACT-agent:42' } }
        'false-success' { return [pscustomobject]@{ success = $false; agentId = $id } }
        'string-success' { return [pscustomobject]@{ success = 'true'; agentId = $id } }
        'missing-id-response' { return [pscustomobject]@{ success = $true } }
        'empty-response' { return $null }
        'array-response' { return @([pscustomobject]@{ success = $true; agentId = $id }, [pscustomobject]@{ success = $true; agentId = $id }) }
        default { return [pscustomobject]@{ success = $true; agentId = $id } }
    }
}
function Remove-Item {
    [CmdletBinding()]
    param([string]$Path, [switch]$Force, [switch]$Recurse)
    Event 'erase'
    if ($Scenario -eq 'cleanup-fails') { throw 'controlled local cleanup failure' }
}

# Execute the actual complete uninstall branch in this disposable child engine.
# All OS, credential, filesystem and network operations above are private doubles;
# production Exit statements, sequencing, catch/cleanup and success stay intact.
$errors = $null
$ast = [System.Management.Automation.Language.Parser]::ParseFile($InstallerPath, [ref]$null, [ref]$errors)
if ($errors.Count) { throw "Installer parse failed: $errors" }
foreach ($name in @('Parse-Bool', 'Remove-PulseAgentRegistration')) {
    $functions = @($ast.FindAll({ param($node)
        $node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name
    }, $true))
    if ($functions.Count -ne 1) { throw "Expected one production function: $name" }
    Invoke-Expression $functions[0].Extent.Text
}
$branches = @($ast.FindAll({ param($node)
    $node -is [System.Management.Automation.Language.IfStatementAst] -and
    $node.Clauses[0].Item1.Extent.Text -eq '$Uninstall'
}, $true))
if ($branches.Count -ne 1) { throw 'Expected one complete production uninstall branch' }
Invoke-Expression $branches[0].Extent.Text
throw 'Production uninstall failed to exit'
