# Libera, lista ou corta o acesso de um amigo ao tunel WireGuard do servidor, e da ou tira a
# permissao de liberador. So serve para quem cuida do servidor: entra pelo apelido vps do
# ~/.ssh/config, com a chave de administrador.
#
# A chave de amigo e a linha "golive-wg ..." que o executavel mostra na primeira vez; a de
# liberador e a linha "golive-liberador ..." que o Liberar-pessoal.ps1 mostra na primeira vez.
#
# uso: .\Liberar-amigo.cmd -Nome joao -Chave 'golive-wg AAAA...='
#      .\Liberar-amigo.cmd -Listar
#      .\Liberar-amigo.cmd -Remover joao
#      .\Liberar-amigo.cmd -Liberador maria -Chave 'golive-liberador ssh-ed25519 AAAA...' [-Limite 20]
#      .\Liberar-amigo.cmd -TirarLiberador maria
#      .\Liberar-amigo.cmd -Liberadores

[CmdletBinding(DefaultParameterSetName = 'Liberar')]
param(
    [Parameter(ParameterSetName = 'Liberar', Mandatory = $true)]
    [string]$Nome,
    [Parameter(ParameterSetName = 'Liberar', Mandatory = $true)]
    [Parameter(ParameterSetName = 'DarLiberador', Mandatory = $true)]
    [string]$Chave,
    [Parameter(ParameterSetName = 'Listar', Mandatory = $true)]
    [switch]$Listar,
    [Parameter(ParameterSetName = 'Remover', Mandatory = $true)]
    [string]$Remover,
    [Parameter(ParameterSetName = 'DarLiberador', Mandatory = $true)]
    [string]$Liberador,
    [Parameter(ParameterSetName = 'DarLiberador')]
    [ValidateRange(0, 9999)]
    [int]$Limite = 20,
    [Parameter(ParameterSetName = 'TirarLiberador', Mandatory = $true)]
    [string]$TirarLiberador,
    [Parameter(ParameterSetName = 'Liberadores', Mandatory = $true)]
    [switch]$Liberadores
)

$ErrorActionPreference = 'Stop'

# A chave chega por mensagem de outra pessoa e termina numa linha de comando do root, entao
# nada segue para o servidor sem passar por estes formatos fechados. O golive-peer confere de
# novo do lado de la.
function Test-Nome($nome) {
    if ($nome -notmatch '^[A-Za-z0-9_-]{1,32}$') {
        throw "nome invalido: use so letras sem acento, numeros, - e _, ate 32"
    }
}

switch ($PSCmdlet.ParameterSetName) {
    'Listar' {
        ssh vps "golive-peer list"
    }
    'Remover' {
        Test-Nome $Remover
        ssh vps "golive-peer remove $Remover"
    }
    'Liberar' {
        Test-Nome $Nome
        if ($Chave.Trim() -notmatch '^(?:golive-wg\s+)?([A-Za-z0-9+/]{42}[AEIMQUYcgkosw048]=)$') {
            throw "isso nao parece a linha que o programa mostrou: ela comeca com golive-wg"
        }
        ssh vps "golive-peer add $Nome $($Matches[1])"
    }
    'DarLiberador' {
        Test-Nome $Liberador
        if ($Chave.Trim() -notmatch '^(?:golive-liberador\s+)?ssh-ed25519\s+(AAAAC3NzaC1lZDI1NTE5[A-Za-z0-9+/]{48})(?:\s.*)?$') {
            throw "isso nao parece a linha do Liberar-pessoal.ps1: ela comeca com golive-liberador ssh-ed25519"
        }
        ssh vps "golive-peer liberador-add $Liberador $($Matches[1]) $Limite"
    }
    'TirarLiberador' {
        Test-Nome $TirarLiberador
        ssh vps "golive-peer liberador-remove $TirarLiberador"
    }
    'Liberadores' {
        ssh vps "golive-peer liberadores"
    }
}
