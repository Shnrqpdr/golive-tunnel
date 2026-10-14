# Problemas comuns

As mensagens abaixo são as que o `golive-tunnel.exe` mostra na caixa vermelha.

### "o tunel ligou, mas nada passou por ele"

O WireGuard não avisa quando o servidor não conhece a chave: os pacotes só ficam sem resposta. Quase sempre a chave ainda não foi liberada; a mensagem já deixa a linha `golive-wg` copiada para mandar de novo. Se ela já aparece no `-List`, confira se o servidor está ligado, se o `wg0` está no ar (`wg show`) e se a porta UDP chega até ele (firewall do provedor, `ufw`).

### Go Live carregando para sempre, ou erro 2012

Quem transmite e quem assiste precisam estar no túnel. Com só um dos dois, o servidor de mídia continua segurando o vídeo. Confira se os dois abriram o Discord pelo `golive-tunnel.exe`, e não pelo atalho normal.

### "este programa foi aberto como administrador"

Aberto como administrador, ele abriria o Discord como administrador também. Abra com duplo clique normal: quando precisa, o próprio programa pede o aviso do Windows.

### "o aviso de administrador foi recusado"

O aviso só aparece na primeira vez, ou quando muda algo do túnel, como a chave, o servidor ou o nome do executável. É quando o programa instala o WireSock e prepara o serviço. Abra de novo e aceite.

### "ja existe um WireSock rodando fora do servico"

Outro programa, ou um `wiresock-client run` aberto à mão, está usando o WireSock. Feche-o antes, porque dois ao mesmo tempo brigam pelo mesmo filtro de rede.

### "a conexao foi bloqueada pela sua propria maquina"

Antivírus ou firewall do Windows barrando o `golive-tunnel.exe`, que não tem assinatura digital. Libere o programa na lista de exceções.

### "este executavel foi compilado sem servidor embutido"

O executável veio de um `go build` comum. Compile com `scripts/build.sh`, com o `.env` preenchido.

### "nao consegui trazer a chave da versao anterior"

O programa achou a chave do antigo `DiscordGoLiveBypass.exe` mas não conseguiu movê-la para `%LOCALAPPDATA%\golive-tunnel`. Ele para em vez de gerar uma chave nova, porque uma chave nova precisaria ser liberada de novo. Confira as permissões das duas pastas; em último caso, apague a antiga e peça para ser liberado outra vez.

### "o tunel funciona, mas a saida nao serve"

O túnel funciona, mas o país de saída está na lista de recusados. O servidor está no Brasil, ou o serviço de geolocalização errou o país; use um servidor fora do Brasil.
