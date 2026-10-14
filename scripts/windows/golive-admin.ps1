# Lets a friend into the server's WireGuard tunnel, lists or removes them, and grants or takes
# away the delegate permission. Only for whoever runs the server: it logs in as root over SSH,
# through the host given in -SshHost (an alias from ~/.ssh/config, or root@address).
#
# A friend's key is the "golive-wg ..." line the executable shows on its first run; a delegate's
# is the "golive-delegate ..." line golive-delegate.ps1 shows on its first run.
#
# usage: .\golive-admin.cmd -Add joao -Key 'golive-wg AAAA...='
#        .\golive-admin.cmd -List
#        .\golive-admin.cmd -Remove joao
#        .\golive-admin.cmd -AddDelegate maria -Key 'golive-delegate ssh-ed25519 AAAA...' [-Limit 20]
#        .\golive-admin.cmd -RemoveDelegate maria
#        .\golive-admin.cmd -ListDelegates
#
# The SSH host defaults to $env:GOLIVE_SSH_HOST, or "vps" when that is not set.

[CmdletBinding(DefaultParameterSetName = 'Help')]
param(
    [Parameter(ParameterSetName = 'Add', Mandatory = $true)]
    [string]$Add,
    [Parameter(ParameterSetName = 'Add', Mandatory = $true)]
    [Parameter(ParameterSetName = 'AddDelegate', Mandatory = $true)]
    [string]$Key,
    [Parameter(ParameterSetName = 'List', Mandatory = $true)]
    [switch]$List,
    [Parameter(ParameterSetName = 'Remove', Mandatory = $true)]
    [string]$Remove,
    [Parameter(ParameterSetName = 'AddDelegate', Mandatory = $true)]
    [string]$AddDelegate,
    [Parameter(ParameterSetName = 'AddDelegate')]
    [ValidateRange(0, 9999)]
    [int]$Limit = 20,
    [Parameter(ParameterSetName = 'RemoveDelegate', Mandatory = $true)]
    [string]$RemoveDelegate,
    [Parameter(ParameterSetName = 'ListDelegates', Mandatory = $true)]
    [switch]$ListDelegates,
    [string]$SshHost = $(if ($env:GOLIVE_SSH_HOST) { $env:GOLIVE_SSH_HOST } else { 'vps' })
)

$ErrorActionPreference = 'Stop'

# The key arrives in someone else's message and ends up on root's command line, so nothing goes
# to the server without matching these closed formats. golive-peer checks again on its side.
function Test-Name($name) {
    if ($name -notmatch '^[A-Za-z0-9_-]{1,32}$') {
        throw "nome invalido: use so letras sem acento, numeros, - e _, ate 32"
    }
}

function Invoke-Server([string]$request) {
    ssh $SshHost "golive-peer $request"
    exit $LASTEXITCODE
}

switch ($PSCmdlet.ParameterSetName) {
    'Help' {
        Write-Host 'uso: .\golive-admin.cmd -Add joao -Key ''golive-wg ...''          libera um amigo'
        Write-Host '     .\golive-admin.cmd -List                                     lista quem tem acesso'
        Write-Host '     .\golive-admin.cmd -Remove joao                              corta um amigo'
        Write-Host '     .\golive-admin.cmd -AddDelegate maria -Key ''golive-delegate ...'' [-Limit 20]'
        Write-Host '                                                                  da a permissao de liberador'
        Write-Host '     .\golive-admin.cmd -RemoveDelegate maria                     tira a permissao de liberador'
        Write-Host '     .\golive-admin.cmd -ListDelegates                            lista os liberadores'
        Write-Host ''
        Write-Host "Entra como root em '$SshHost' pelo SSH. Para outro servidor, use -SshHost ou a variavel GOLIVE_SSH_HOST."
    }
    'List' { Invoke-Server 'list' }
    'Remove' {
        Test-Name $Remove
        Invoke-Server "remove $Remove"
    }
    'Add' {
        Test-Name $Add
        if ($Key.Trim() -notmatch '^(?:golive-wg\s+)?([A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=)$') {
            throw "isso nao parece a linha que o programa mostrou: ela comeca com golive-wg"
        }
        Invoke-Server "add $Add $($Matches[1])"
    }
    'AddDelegate' {
        Test-Name $AddDelegate
        # golive-liberador is the prefix from before the rename; a line made then still works.
        if ($Key.Trim() -notmatch '^(?:golive-(?:delegate|liberador)\s+)?ssh-ed25519\s+(AAAAC3NzaC1lZDI1NTE5[A-Za-z0-9+/]{48})(?:\s.*)?$') {
            throw "isso nao parece a linha do golive-delegate: ela comeca com golive-delegate ssh-ed25519"
        }
        Invoke-Server "delegate-add $AddDelegate $($Matches[1]) $Limit"
    }
    'RemoveDelegate' {
        Test-Name $RemoveDelegate
        Invoke-Server "delegate-remove $RemoveDelegate"
    }
    'ListDelegates' { Invoke-Server 'delegates' }
}
