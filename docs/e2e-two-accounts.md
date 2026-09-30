# e2e com duas contas

Read when: usar o wacli como runtime de testes e2e de um produto que conversa por WhatsApp, com duas pessoas (duas contas pareadas) falando com um bot.

Cada pessoa é uma conta nomeada do wacli: um store próprio (`session.db`, `wacli.db`, `LOCK`, `.send.sock`) e um processo `sync --follow` próprio. Os dois daemons rodam ao mesmo tempo na mesma máquina sem se bloquear. Os comandos de envio e de grupo são delegados ao daemon da conta pelo socket; as leituras abrem só o `wacli.db` e funcionam com `--read-only`.

Os exemplos usam placeholders: `+55DD9XXXXXXX1` (pessoa 1), `+55DD9XXXXXXX2` (pessoa 2) e `+55DD9XXXXXXX0` (bot). Nenhum segredo precisa ficar em arquivo.

## Isolar o registro de contas

`wacli accounts` grava o registro em `<base>/config.yaml`, onde `<base>` é a raiz de estado padrão (`~/.local/state/wacli` no Linux). `--store` e `WACLI_STORE_DIR` não mudam esse caminho. O primeiro `accounts add` também define `default_account` quando ainda não há um, o que muda o store que um `wacli` sem flags usa. Para o e2e não tocar em uma instalação pessoal, aponte a raiz para um diretório do e2e antes de qualquer comando `accounts` ou `--account`:

```bash
export E2E_ROOT="$PWD/.e2e-wacli"
export XDG_STATE_HOME="$E2E_ROOT/state"   # Linux: o registro vai para $XDG_STATE_HOME/wacli/config.yaml
wacli accounts add pessoa1 --no-auth
wacli accounts add pessoa2 --no-auth
wacli --json accounts list
```

Os stores ficam em `$XDG_STATE_HOME/wacli/accounts/pessoa1` e `.../pessoa2`. No macOS `XDG_STATE_HOME` é ignorado; lá use `--store "$E2E_ROOT/pessoa1"` em cada comando no lugar de `--account pessoa1` (as duas formas não se combinam). Veja [accounts](accounts.md).

## Parear cada conta

Pareamento por número: o wacli imprime um código de 8 caracteres em stderr; digite-o no celular da pessoa em WhatsApp > Dispositivos conectados > Conectar dispositivo > Conectar com número de telefone. Mantenha o comando rodando até autenticar.

```bash
WACLI_SYNC_MAX_MESSAGES=5000 wacli --account pessoa1 auth --phone "+55DD9XXXXXXX1"
WACLI_SYNC_MAX_MESSAGES=5000 wacli --account pessoa2 auth --phone "+55DD9XXXXXXX2"
```

Pareamento por QR em texto: `--qr-format text` escreve o payload cru do QR em stdout, para outro programa renderizar (por exemplo `qrencode -t ansiutf8`). Com `--events`, o código de pareamento e o QR saem como eventos NDJSON `pair_code` e `qr_code` em stderr.

```bash
wacli --account pessoa1 auth --qr-format text
wacli --account pessoa1 --events auth --phone "+55DD9XXXXXXX1" 2> pair.ndjson
wacli --account pessoa1 --json auth status
```

`WACLI_SYNC_MAX_MESSAGES` limita o histórico baixado no bootstrap. Veja [auth](auth.md).

## Subir os dois daemons

```bash
for p in pessoa1 pessoa2; do
  wacli --account "$p" --json sync --follow --max-messages 20000 \
    > "$E2E_ROOT/$p.ndjson" 2> "$E2E_ROOT/$p.err" &
  echo $! > "$E2E_ROOT/$p.pid"
done
# Pronto quando cada .ndjson tiver o evento ready.
until grep -q '"event":"ready"' "$E2E_ROOT/pessoa1.ndjson" && grep -q '"event":"ready"' "$E2E_ROOT/pessoa2.ndjson"; do sleep 0.2; done
```

O stdout de cada daemon é NDJSON (`ready`, `message` com botões e legenda, `receipt`, `chat_presence`, avisos). Para parar: `kill "$(cat "$E2E_ROOT/pessoa1.pid")"` (SIGTERM). Veja [sync](sync.md).

## Comandos que o e2e usa

Com os daemons rodando:

| Necessidade | Comando |
| --- | --- |
| texto para telefone, JID ou grupo | `wacli --account pessoa1 --json send text --to +55DD9XXXXXXX0 --message "oi"` (também `--to 55DD9XXXXXXX0@s.whatsapp.net` ou `--to <grupo>@g.us`) |
| imagem ou PDF com legenda | `wacli --account pessoa1 --json send file --to <grupo>@g.us --file recibo.jpg --caption "mercado 45,90"` (PNG/JPG saem como foto, PDF como documento; `--as document` força documento) |
| criar grupo e obter o JID | `wacli --account pessoa1 --json groups create --name "E2E casal" --user +55DD9XXXXXXX2 --user +55DD9XXXXXXX0` → `.data.JID`; `.data.Participants[]` traz `JID`, `PhoneNumber` e `LID` de cada participante |
| listar participantes | `wacli --account pessoa1 --read-only --json groups participants list --jid <grupo>@g.us` |
| sair do grupo | `wacli --account pessoa1 --json groups leave --jid <grupo>@g.us` |
| ler mensagens recebidas | `wacli --account pessoa1 --read-only --json messages list --chat <grupo>@g.us --sender +55DD9XXXXXXX0 --after 2026-01-01T12:00:00Z --asc` |
| esperar resposta | `wacli --account pessoa1 --read-only --json --timeout 30s messages wait --chat <grupo>@g.us --after-id <id enviado> --from-them` |
| telefone ↔ LID | `wacli --account pessoa1 --read-only --json contacts show --jid 55DD9XXXXXXX0@s.whatsapp.net` → `lid` (aceita também `<lid>@lid`) |
| tocar em botão | `wacli --account pessoa1 --json send select --to <grupo>@g.us --id <msg com botões> --label "Aceito"` |
| baixar arquivo recebido | `wacli --account pessoa1 --read-only --json media download --chat <grupo>@g.us --id <msg> --output "$E2E_ROOT/out/"` |

Cada mensagem em JSON traz `Text` (com os links, inclusive `wa.me`), `Buttons[]` (`display_text`, `id`, `type`), `MediaType`, `MediaCaption`, `Filename`, `SenderJID`, `FromMe` e `Timestamp`. `--sender` e `--chat` aceitam telefone ou JID e casam as formas telefone e `@lid` da mesma pessoa quando a sessão conhece o mapeamento; o bot que fala por `@lid` é encontrado pelo número dele. Veja [messages](messages.md).

`messages wait --after-id` é a forma precisa de esperar uma resposta: casa as mensagens guardadas depois da mensagem âncora, mesmo no mesmo segundo. Se a âncora é mensagem de outra conta que ainda não chegou neste store, a espera continua até ela e a resposta chegarem. `--count N` espera N mensagens. Sem conta real, a espera leva no máximo `--poll-interval` (padrão 200ms) mais ~10ms depois que o daemon grava a mensagem.

## Roteiro mínimo

```bash
set -euo pipefail
W() { wacli --account "$1" --json "${@:2}"; }
R() { wacli --account "$1" --read-only --json --timeout 60s "${@:2}"; }
BOT=+55DD9XXXXXXX0; P2=+55DD9XXXXXXX2

# 1. Pessoa 1 escreve ao bot no privado e espera a resposta.
id=$(W pessoa1 send text --to "$BOT" --message "oi" | jq -r .data.id)
R pessoa1 messages wait --chat "$BOT" --after-id "$id" --from-them | jq -r '.data.messages[].Text'

# 2. Pessoa 1 cria o grupo com a pessoa 2 e o bot.
t=$(date -u +%Y-%m-%dT%H:%M:%SZ)
G=$(W pessoa1 groups create --name "E2E casal" --user "$P2" --user "$BOT" | jq -r .data.JID)

# 3. O bot pede o aceite no grupo; cada pessoa responde (botão ou texto, conforme o bot).
msg=$(R pessoa1 messages wait --chat "$G" --sender "$BOT" --after "$t" | jq -r '.data.messages[0].MsgID')
W pessoa1 send select --to "$G" --id "$msg" --label "Aceito"
R pessoa2 messages wait --chat "$G" --sender "$BOT" --after "$t" >/dev/null   # o mesmo pedido chegou na pessoa 2
W pessoa2 send select --to "$G" --id "$msg" --label "Aceito"

# 4. Gasto por texto, foto e PDF; cada envio espera a resposta do bot.
reply() { R pessoa1 messages wait --chat "$G" --sender "$BOT" --after-id "$1" | jq '.data.messages[0] | {Text, Buttons, MediaType, MediaCaption}'; }
reply "$(W pessoa1 send text --to "$G" --message "mercado 45,90" | jq -r .data.id)"
reply "$(W pessoa1 send file --to "$G" --file recibo.jpg --caption "padaria 12,00" | jq -r .data.id)"
reply "$(W pessoa1 send file --to "$G" --file fatura.pdf --caption "luz setembro" | jq -r .data.id)"

# 5. Exportar: o bot responde com um arquivo; baixe-o.
id=$(W pessoa2 send text --to "$G" --message "exportar" | jq -r .data.id)
exp=$(R pessoa2 messages wait --chat "$G" --sender "$BOT" --after-id "$id" | jq -r '.data.messages[0].MsgID')
R pessoa2 media download --chat "$G" --id "$exp" --output "$E2E_ROOT/out/"

# 6. Apagar e sair do grupo.
id=$(W pessoa1 send text --to "$G" --message "apagar" | jq -r .data.id)
R pessoa1 messages wait --chat "$G" --sender "$BOT" --after-id "$id"
W pessoa1 groups leave --jid "$G"
W pessoa2 groups leave --jid "$G"
```

As frases enviadas ao bot, os rótulos de botão e o formato da exportação dependem do produto; troque-os pelos do bot testado.

## Ensaio sem WhatsApp

`sync --follow --mock` (ou `WACLI_MOCK=1`) sobe o daemon sem conta pareada, com o mesmo socket. `send text`, `send file` e `send select` são gravados localmente como mensagens suas; `wacli --account pessoa2 --json sync inject --chat <grupo>@g.us --sender 55DD9XXXXXXX0@s.whatsapp.net --message "..." --buttons "Aceito:accept,Recusar:reject"` simula uma mensagem recebida. As contas mock não trocam mensagens entre si nem com o bot, e `groups create` é recusado. Serve para ensaiar o encanamento do roteiro (socket, espera, JSON), não o comportamento do bot.

## Limites

- Só um processo por store segura o `LOCK`. Com o daemon rodando, `groups info`, `groups participants add|remove` e os outros comandos de grupo ao vivo, exceto `create` e `leave`, falham com store bloqueado; pare o daemon para usá-los.
- O mapeamento telefone ↔ LID vem da sessão do whatsmeow. Antes da sessão ver o contato (mensagem, grupo ou `contacts refresh`), `contacts show` não traz `lid` e `--sender` casa só a forma informada.
- Pareamento, entrega real, tempo de resposta do bot e aceite de grupo só se verificam com contas pareadas.
