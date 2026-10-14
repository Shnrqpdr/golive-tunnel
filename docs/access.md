# Liberar, listar e cortar acessos

Cada amigo tem uma chave WireGuard gerada no computador dele. A chave privada nunca sai de lá: o amigo manda só a pública, na linha `golive-wg ...` que o executável mostra na primeira vez. Liberar é registrar essa chave no servidor com um nome.

O endereço do amigo dentro do túnel sai da própria chave (`10.66.x.y`, com `x` e `y` tirados do SHA256 dela), com a mesma conta no executável e no `golive-peer`. Ninguém precisa combinar IP.

## Pelo Windows: `golive-admin`

O [`scripts/windows/golive-admin.cmd`](../scripts/windows/golive-admin.cmd) entra no servidor como root pelo SSH. Por padrão ele usa o host `vps` do seu `~/.ssh/config`; para outro, passe `-SshHost` ou defina a variável `GOLIVE_SSH_HOST`.

```powershell
.\golive-admin.cmd -Add joao -Key 'golive-wg AAAA...='
.\golive-admin.cmd -List
.\golive-admin.cmd -Remove joao
.\golive-admin.cmd -List -SshHost root@203.0.113.10
```

Um exemplo de `~/.ssh/config` (no Windows, `C:\Users\<você>\.ssh\config`):

```
Host vps
    HostName 203.0.113.10
    User root
    IdentityFile ~/.ssh/id_ed25519
```

O `.cmd` só chama o `.ps1` com a política de scripts liberada para aquela execução: o Windows, por padrão, recusa um `.ps1` digitado direto. A linha que o amigo manda é validada aqui e de novo no servidor, porque termina numa linha de comando do root.

O `-List` mostra o endereço de cada um, quem liberou e o último contato:

```
joao                 10.66.125.59    por dono     ultimo contato 13/10 21:40
pedro                10.66.77.147    por maria    nunca conectou
```

## Por qualquer terminal: `golive-peer`

No servidor, ou por `ssh root@servidor ...`:

```sh
golive-peer add joao AAAA...=      # a chave sem o prefixo golive-wg
golive-peer list
golive-peer remove joao
```

## Liberadores

Um liberador é um amigo de confiança que libera outros sozinho, sem acesso ao servidor. Ele libera até o limite que você der, vê a lista inteira e só corta quem ele mesmo liberou.

1. Mande para ele o `dist/golive-delegate-<versão>.zip`. O `scripts/build.sh` monta esse kit com o endereço e a identidade SSH do servidor tirados do `.env`; ele não vai no zip dos amigos comuns.
2. Ele roda `.\golive-delegate.cmd` uma vez. O script gera uma chave SSH só para isso e mostra uma linha `golive-delegate ssh-ed25519 ...`, que ele te manda.
3. Você dá a permissão:

   ```powershell
   .\golive-admin.cmd -AddDelegate maria -Key 'golive-delegate ssh-ed25519 AAAA...' -Limit 20
   ```

4. Daí em diante ele usa `.\golive-delegate.cmd -Add`, `-List` e `-Remove` como você.

Você acompanha com `-ListDelegates` e tira a permissão com `-RemoveDelegate maria`. Quem ele liberou continua com acesso até você cortar.

### Como isso fica seguro

- Todos os liberadores entram pelo usuário `golive-delegate`, que não tem senha.
- Cada chave tem no `authorized_keys` a opção `restrict` (sem terminal, sem encaminhamento de porta) e um comando forçado, `golive-delegate-login <nome>`. O SSH ignora o que for pedido e só roda esse script, com o nome gravado por você: quem conecta não escolhe quem é.
- O script aceita só palavras simples, registra o pedido no journal (`journalctl -t golive-delegate`) e chama `sudo golive-peer as <nome>`, a única coisa que o `sudo` permite a esse usuário.
- O `golive-peer as` aceita só `add`, `remove` e `list`, confere o limite e recusa cortar quem foi liberado por outra pessoa.
- O kit do liberador fixa a identidade SSH do servidor e recusa conectar se ela mudar.

Pelo `golive-peer`, no servidor:

```sh
golive-peer delegate-add maria AAAAC3NzaC1lZDI1NTE5... 20    # a chave sem o ssh-ed25519
golive-peer delegates
golive-peer delegate-remove maria
```
