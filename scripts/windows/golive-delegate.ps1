# Lets friends into the server's tunnel, lists or removes them, as a delegate: you add up to your
# limit, see everyone and only remove whoever you added yourself.
#
# This file is a template: scripts/build.sh fills in the server and its SSH identity and puts the
# copy that goes to the delegate in dist/.
#
# usage: .\golive-delegate.cmd                                  (first time: creates your key)
#        .\golive-delegate.cmd -Add joao -Key 'golive-wg AAAA...='
#        .\golive-delegate.cmd -List
#        .\golive-delegate.cmd -Remove joao

[CmdletBinding(DefaultParameterSetName = 'Start')]
param(
    [Parameter(ParameterSetName = 'Add', Mandatory = $true)]
    [string]$Add,
    [Parameter(ParameterSetName = 'Add', Mandatory = $true)]
    [string]$Key,
    [Parameter(ParameterSetName = 'List', Mandatory = $true)]
    [switch]$List,
    [Parameter(ParameterSetName = 'Remove', Mandatory = $true)]
    [string]$Remove
)

$ErrorActionPreference = 'Stop'

# Pinned identity: the script only talks to this server and refuses an impostor at the address.
$server = '__SERVER__'
$hostKey = '__HOST_KEY__'
if ($server.StartsWith('__')) { throw 'Este e o modelo: use o golive-delegate.ps1 gerado pelo scripts/build.sh.' }

$sshDir = Join-Path $env:USERPROFILE '.ssh'
$sshKey = Join-Path $sshDir 'golive_delegate'
New-Item -ItemType Directory -Force $sshDir | Out-Null
Set-Content -LiteralPath (Join-Path $sshDir 'known_hosts_golive') -Encoding ascii -Value "$server $hostKey"

function Show-Line {
    $line = 'golive-delegate ' + (Get-Content -LiteralPath "$sshKey.pub" -Raw).Trim()
    Set-Clipboard -Value $line
    Write-Host ''
    Write-Host 'Mande a linha abaixo para quem cuida do servidor (ja esta copiada). Nao e senha:' -ForegroundColor Green
    Write-Host ''
    Write-Host "    $line"
    Write-Host ''
}

if (-not (Test-Path -LiteralPath $sshKey)) {
    # The OpenSSH client is an optional Windows feature; without it Start-Process fails with an
    # error that means nothing to a friend.
    if (-not (Get-Command ssh-keygen -ErrorAction SilentlyContinue)) {
        throw 'falta o Cliente OpenSSH do Windows: Configuracoes > Sistema > Recursos opcionais > Adicionar um recurso > Cliente OpenSSH.'
    }
    # The arguments go as a single string because PowerShell 5.1 drops empty arguments, and the
    # -N "" is precisely the empty passphrase. On the server the key only adds, lists and removes.
    $keygen = Start-Process ssh-keygen -ArgumentList "-q -t ed25519 -N `"`" -C golive-delegate -f `"$sshKey`"" -NoNewWindow -Wait -PassThru
    if ($keygen.ExitCode -ne 0) { throw "o ssh-keygen falhou com codigo $($keygen.ExitCode)" }
    Show-Line
    Write-Host 'Quando avisarem que voce virou liberador, use -Add e -Key para liberar alguem.'
    exit 0
}

# The name comes from someone else's message; the server checks again, but refusing here gives a
# better message than "pedido recusado".
function Test-Name($name) {
    if ($name -notmatch '^[A-Za-z0-9_-]{1,32}$') {
        throw "nome invalido: use so letras sem acento, numeros, - e _, ate 32"
    }
}

# With ~ ssh finds the files even when the user folder has a space in its name.
function Invoke-Server([string]$request) {
    ssh -o 'IdentityFile=~/.ssh/golive_delegate' -o IdentitiesOnly=yes `
        -o 'UserKnownHostsFile=~/.ssh/known_hosts_golive' -o StrictHostKeyChecking=yes `
        -o BatchMode=yes "golive-delegate@$server" $request
    exit $LASTEXITCODE
}

switch ($PSCmdlet.ParameterSetName) {
    'Start' {
        Show-Line
        Write-Host 'Para liberar alguem:  .\golive-delegate.cmd -Add joao -Key ''golive-wg ...'''
        Write-Host 'Para ver a lista:     .\golive-delegate.cmd -List'
        Write-Host 'Para cortar:          .\golive-delegate.cmd -Remove joao'
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
}
