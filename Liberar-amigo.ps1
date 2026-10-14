# Libera, lista ou corta o acesso de um amigo ao tunel WireGuard do servidor. So serve para
# quem cuida do servidor: entra pelo apelido vps do ~/.ssh/config, com a chave de administrador.
#
# A chave e a linha "golive-wg ..." que o executavel mostra ao amigo na primeira vez.
#
# uso: .\Liberar-amigo.ps1 -Nome joao -Chave 'golive-wg AAAA...='
#      .\Liberar-amigo.ps1 -Listar
#      .\Liberar-amigo.ps1 -Remover joao

[CmdletBinding(DefaultParameterSetName = 'Liberar')]
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
}
