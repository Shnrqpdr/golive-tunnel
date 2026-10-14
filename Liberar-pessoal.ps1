# Libera, lista ou corta amigos no tunel do servidor, como liberador: voce libera ate o seu
# limite, ve todo mundo e corta so quem voce mesmo liberou.
#
# Este arquivo e um modelo: o compilar-amigos.sh troca o servidor e a identidade dele e gera
# a versao que vai para o liberador, em dist/.
#
# uso: .\Liberar-pessoal.cmd                                  (primeira vez: gera a sua chave)
#      .\Liberar-pessoal.cmd -Nome joao -Chave 'golive-wg AAAA...='
#      .\Liberar-pessoal.cmd -Listar
#      .\Liberar-pessoal.cmd -Remover joao

[CmdletBinding(DefaultParameterSetName = 'Inicio')]
param(
    [Parameter(ParameterSetName = 'Liberar', Mandatory = $true)]
    [string]$Nome,
    [Parameter(ParameterSetName = 'Liberar', Mandatory = $true)]
    [string]$Chave,
    [Parameter(ParameterSetName = 'Listar', Mandatory = $true)]
    [switch]$Listar,
    [Parameter(ParameterSetName = 'Remover', Mandatory = $true)]
    [string]$Remover
)

$ErrorActionPreference = 'Stop'

# Identidade fixada: o script so conversa com este servidor e recusa um impostor no endereco.
$servidor = '__SERVIDOR__'
$identidade = '__IDENTIDADE__'
if ($servidor.StartsWith('__')) { throw 'Este e o modelo: use o Liberar-pessoal.ps1 gerado pelo compilar-amigos.sh.' }

$pasta = Join-Path $env:USERPROFILE '.ssh'
$chaveSsh = Join-Path $pasta 'golive_liberador'
New-Item -ItemType Directory -Force $pasta | Out-Null
Set-Content -LiteralPath (Join-Path $pasta 'known_hosts_golive') -Encoding ascii -Value "$servidor $identidade"

function Show-Linha {
    $linha = 'golive-liberador ' + (Get-Content -LiteralPath "$chaveSsh.pub" -Raw).Trim()
    Set-Clipboard -Value $linha
    Write-Host ''
    Write-Host 'Mande a linha abaixo para quem cuida do servidor (ja esta copiada). Nao e senha:' -ForegroundColor Green
    Write-Host ''
    Write-Host "    $linha"
    Write-Host ''
}

if (-not (Test-Path -LiteralPath $chaveSsh)) {
    # Os argumentos vao numa string so porque o PowerShell 5.1 descarta argumento vazio, e o
    # -N "" e justamente a senha vazia. Do lado do servidor a chave so libera, lista e corta.
    $gerador = Start-Process ssh-keygen -ArgumentList "-q -t ed25519 -N `"`" -C golive-liberador -f `"$chaveSsh`"" -NoNewWindow -Wait -PassThru
    if ($gerador.ExitCode -ne 0) { throw "o ssh-keygen falhou com codigo $($gerador.ExitCode)" }
    Show-Linha
    Write-Host 'Quando avisarem que voce virou liberador, use -Nome e -Chave para liberar alguem.'
    exit 0
}

# O nome chega de mensagem de outra pessoa; o servidor confere de novo, mas recusar aqui da uma
# mensagem melhor do que um "pedido recusado".
function Test-Nome($nome) {
    if ($nome -notmatch '^[A-Za-z0-9_-]{1,32}$') {
        throw "nome invalido: use so letras sem acento, numeros, - e _, ate 32"
    }
}

# Com ~ o ssh acha os arquivos mesmo quando a pasta do usuario tem espaco no nome.
function Invoke-Servidor([string]$pedido) {
    ssh -o 'IdentityFile=~/.ssh/golive_liberador' -o IdentitiesOnly=yes `
        -o 'UserKnownHostsFile=~/.ssh/known_hosts_golive' -o StrictHostKeyChecking=yes `
        -o BatchMode=yes "liberador@$servidor" $pedido
}

switch ($PSCmdlet.ParameterSetName) {
    'Inicio' {
        Show-Linha
        Write-Host 'Para liberar alguem:  .\Liberar-pessoal.cmd -Nome joao -Chave ''golive-wg ...'''
        Write-Host 'Para ver a lista:     .\Liberar-pessoal.cmd -Listar'
        Write-Host 'Para cortar:          .\Liberar-pessoal.cmd -Remover joao'
    }
    'Listar' { Invoke-Servidor 'list' }
    'Remover' {
        Test-Nome $Remover
        Invoke-Servidor "remove $Remover"
    }
    'Liberar' {
        Test-Nome $Nome
        if ($Chave.Trim() -notmatch '^(?:golive-wg\s+)?([A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=)$') {
            throw "isso nao parece a linha que o programa mostrou: ela comeca com golive-wg"
        }
        Invoke-Servidor "add $Nome $($Matches[1])"
    }
}
