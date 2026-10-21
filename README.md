# Telegram File Stream Bot

![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Telegram](https://img.shields.io/badge/Telegram-26A5E4?style=for-the-badge&logo=telegram&logoColor=white)
![gotgproto](https://img.shields.io/badge/gotgproto-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)
![gotd](https://img.shields.io/badge/gotd-2CA5E0?style=for-the-badge&logo=telegram&logoColor=white)
![Gin](https://img.shields.io/badge/Gin-008ECF?style=for-the-badge&logo=gin&logoColor=white)
![SQLite](https://img.shields.io/badge/SQLite-003B57?style=for-the-badge&logo=sqlite&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-066DA5?style=for-the-badge&logo=docker&logoColor=white)

Bot para transformar arquivos armazenados no Telegram em links HTTP públicos. O arquivo fica guardado em um canal privado do Telegram e o serviço entrega seu conteúdo por streaming, sem precisar baixar o arquivo inteiro antes de começar a reprodução ou o download.

Este repositório é uma adaptação do [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot), modificada para atender a uma instalação própria e integrar o envio de arquivos ao serviço que hospeda o bot.

## ✨ Funcionalidades implementadas

- Recebe documentos e fotos enviados ao bot por mensagem privada e os encaminha ao canal de armazenamento.
- Gera um link HTTP para cada arquivo, com opções para baixar ou abrir em streaming quando o tipo de mídia é compatível.
- Transmite arquivos com suporte a requisições HTTP `Range`, necessárias para avançar e retroceder em vídeos e áudios.
- Oferece a rota `POST /api/upload`, protegida por token, para integração com outros serviços. O limite atual dessa rota é 50 MB por arquivo.
- Permite restringir quem pode enviar arquivos ao bot usando uma lista de IDs numéricos do Telegram (`ALLOWED_USERS`).
- Mantém metadados em cache durante a execução e sessões locais em SQLite; não requer banco de dados externo.
- Usa Docker Compose para compilar e executar o serviço.

Os links gerados funcionam como credenciais: qualquer pessoa que tenha o endereço completo pode acessar o arquivo. A lista `ALLOWED_USERS` controla quem pode enviar arquivos ao bot, mas não restringe o acesso a links já gerados.

## 🧰 Pré-requisitos

- Uma conta no Telegram.
- Docker e Docker Compose.
- Um servidor ou computador que mantenha o serviço em execução. Para acesso externo, configure um domínio com HTTPS e um proxy reverso apontando para a porta local do serviço.
- Credenciais da API do Telegram (`API_ID` e `API_HASH`), obtidas em [my.telegram.org](https://my.telegram.org).

## 🤖 Configuração no Telegram

### 1. Crie seu bot pessoal

1. No Telegram, abra uma conversa com [@BotFather](https://t.me/BotFather).
2. Envie `/newbot`, escolha o nome de exibição e um nome de usuário terminado em `bot`.
3. Guarde o token fornecido pelo BotFather. Ele será usado como `BOT_TOKEN`; não o compartilhe nem o publique.
4. Abra a conversa com o bot que acabou de criar e envie `/start`.

### 2. Crie o canal de armazenamento

1. Crie um canal no Telegram. Recomendamos deixá-lo **privado**, pois é nele que o bot armazenará os arquivos.
2. Adicione o seu bot como administrador do canal e habilite a permissão para publicar mensagens.
3. Copie o ID do canal no formato `-1001234567890` (incluindo o prefixo `-100`) e use esse valor em `LOG_CHANNEL`.

O serviço envia os arquivos para esse canal e depois os lê para atender aos links. Mantenha o bot como administrador e não apague as mensagens que ainda precisam ser acessadas.

### 3. Obtenha as credenciais da API

1. Acesse [my.telegram.org](https://my.telegram.org) e entre com sua conta.
2. Abra **API development tools** e crie uma aplicação, se ainda não houver uma.
3. Copie `api_id` para `API_ID` e `api_hash` para `API_HASH`.

## 🛠️ Instalação e execução

Na raiz do repositório, crie o arquivo de configuração local:

```sh
cp fsb.sample.env fsb.env
chmod 600 fsb.env
```

Edite `fsb.env` e informe os valores obrigatórios:

```dotenv
API_ID=seu_api_id
API_HASH=seu_api_hash
BOT_TOKEN=token_do_botfather
LOG_CHANNEL=-1001234567890
```

Configure também a URL pública que será inserida nos links. Em produção, use o domínio servido pelo seu proxy reverso, incluindo `https://`:

```dotenv
HOST=https://arquivos.exemplo.com
```

Para restringir quem pode enviar arquivos ao bot, informe os IDs numéricos dos usuários separados por vírgula:

```dotenv
ALLOWED_USERS=123456789,987654321
```

Se deixar `ALLOWED_USERS` vazio, qualquer pessoa poderá enviar arquivos ao bot. Isso não torna privados os links que já tiverem sido gerados.

Inicie o serviço:

```sh
docker compose up -d --build
```

O Compose publica a porta `8080` apenas em `127.0.0.1`, adequada para uso atrás de um proxy reverso local. Configure o proxy para encaminhar o tráfego HTTPS do domínio ao serviço nessa porta. Para verificar se o processo está no ar, consulte `https://arquivos.exemplo.com/`; a resposta inclui status, versão e tempo de atividade.

Para acompanhar os registros:

```sh
docker compose logs -f
```

## 📤 Envio pelo endpoint de integração

Além do envio pelo Telegram, outro serviço pode enviar um arquivo usando `POST /api/upload`. Defina um segredo forte em `UPLOAD_TOKEN` no `fsb.env` e envie-o no cabeçalho `X-Upload-Token`. O arquivo deve ser um campo multipart chamado `file`.

Exemplo:

```sh
curl -X POST 'https://arquivos.exemplo.com/api/upload' \
  -H 'X-Upload-Token: seu_segredo' \
  -F 'file=@./arquivo.pdf'
```

Em caso de sucesso, a API responde com status `201 Created` e um JSON contendo a URL pública do arquivo. A rota rejeita chamadas se `UPLOAD_TOKEN` não estiver configurado e aceita arquivos de até 50 MB.

## ⚙️ Variáveis de ambiente

| Variável | Obrigatória | Descrição |
| --- | :---: | --- |
| `API_ID` | Sim | ID da aplicação Telegram, obtido em `my.telegram.org`. |
| `API_HASH` | Sim | Hash da aplicação Telegram. |
| `BOT_TOKEN` | Sim | Token do bot criado pelo BotFather. |
| `LOG_CHANNEL` | Sim | ID do canal de armazenamento, no formato `-100...`. |
| `HOST` | Recomendado | URL base pública usada nos links; em produção, use HTTPS. |
| `ALLOWED_USERS` | Não | IDs numéricos autorizados a enviar arquivos, separados por vírgula. Vazio permite todos. |
| `UPLOAD_TOKEN` | Para integração | Segredo exigido pela rota `POST /api/upload`. |
| `PORT` | Não | Porta HTTP interna. Padrão: `8080`. |
| `HASH_LENGTH` | Não | Tamanho do hash dos links, limitado automaticamente ao intervalo de 24 a 32. |
| `DEV` | Não | Ativa o modo de desenvolvimento. Padrão: `false`. |
| `USE_SESSION_FILE` | Não | Usa sessões locais em arquivo. Padrão: `true`. |
| `STREAM_CONCURRENCY` | Não | Número de blocos buscados em paralelo durante o streaming. Padrão: `4`. |
| `STREAM_BUFFER_COUNT` | Não | Quantidade de blocos pré-carregados. Padrão: `8`. |
| `STREAM_TIMEOUT_SEC` | Não | Tempo limite para buscar cada bloco, em segundos. Padrão: `30`. |
| `STREAM_MAX_RETRIES` | Não | Tentativas para buscar novamente um bloco com falha. Padrão: `3`. |

`fsb.env` contém segredos e está excluído do Git. Não publique esse arquivo, o token do bot, `API_HASH`, `UPLOAD_TOKEN` nem URLs completas de arquivos.

## 💻 Tecnologias

- Go e [gotgproto](https://github.com/celestix/gotgproto) para integração com o Telegram.
- [Gin](https://github.com/gin-gonic/gin) para as rotas HTTP.
- SQLite para persistência local das sessões dos bots.
- Docker e Docker Compose para build e execução.

## 📄 Licença e créditos

Este projeto deriva do [TG-FileStreamBot](https://github.com/EverythingSuckz/TG-FileStreamBot) e mantém a licença [GNU Affero General Public License v3.0](LICENSE). Consulte o arquivo de licença para os termos de uso, distribuição e modificação.

Créditos do projeto original:

- [@celestix](https://github.com/celestix), pelo [gotgproto](https://github.com/celestix/gotgproto).
- [@divyam234](https://github.com/divyam234), pelo projeto [Teldrive](https://github.com/divyam234/teldrive).
- [@krau](https://github.com/krau), pelo suporte a imagens.
