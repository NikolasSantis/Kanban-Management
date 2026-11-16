# kanban_management

## Configuração

A API lê a configuração de variáveis de ambiente. O arquivo `.env` é opcional: se existir na raiz, é carregado; variáveis já definidas no ambiente têm precedência.

```bash
cp .env.example .env
```

Preencha ao menos `MONGODB_URI`, `DATABASE_NAME` e `JWT_SECRET` (mínimo 32 caracteres). Todas as variáveis estão documentadas em `.env.example`. Depois:

```bash
go run ./cmd/api
```


## Testes

```bash
go test ./...                       # unitários; a integração é pulada (SKIP) sem MONGODB_URI
go test ./internal/tests/... -v     # integração HTTP (precisa do banco de teste)
go test -cover ./internal/services/...
```

Os testes de integração usam o `TEST_DATABASE_NAME` do `.env` (o nome precisa terminar em `_test` e ser diferente de `DATABASE_NAME`; senão a suíte aborta). Cada pacote de teste usa o seu próprio banco, derivado desse nome (ex.: `<nome>_integration_test`), e todas as coleções são apagadas no final. O banco principal nunca é usado. Sem `MONGODB_URI`, os testes de integração aparecem como SKIP, e a cobertura de `internal/services` fica baixa porque os testes dele também dependem do banco.


## Paginação

Todas as listagens aceitam `?page=` (padrão 1) e `?limit=` (padrão 20, máximo 100: valores maiores viram 100; `page` menor que 1 vira 1; valores não numéricos devolvem 400 `"Invalid pagination params"`).

> **Mudança que quebra compatibilidade:** `data` deixou de ser um array nas rotas de listagem.

```json
GET /board/project/<id>?page=2&limit=10

{
  "success": true,
  "data": {
    "items": [ ... ],
    "pagination": { "page": 2, "limit": 10, "total": 25, "total_pages": 3, "has_next": true }
  },
  "error": null
}
```

`items` é sempre um array (`[]`, nunca `null`). A ordenação é determinística (desempate por `_id`).

**Cards** (`GET /card/board/:board_id`) são paginados por coluna, porque um kanban pagina cada coluna separadamente:

- `?column_id=todo&page=2` devolve `{items, pagination}` só dessa coluna.
- Sem `column_id`, devolve a mesma página de cada coluna ativa: `{"columns": {"todo": {items, pagination}, "doing": {...}}}`.

Fora da paginação ficam `GET /card/project/:project_id/due` (visão de calendário, já limitada a 90 dias).


## Filtros e ordenação

Todos opcionais, combinados com E. Vários valores no mesmo filtro (separados por vírgula, máx. 20) são OU, exceto `label_ids`, que por padrão exige todas.

**Cards** (`GET /card/board/:board_id` e `GET /card/project/:project_id/search`). Com qualquer filtro, `q` ou `sort`, `/card/board/:board_id` responde uma lista plana paginada; sem eles, agrupa por coluna.

| Param | Exemplo | Regra |
|---|---|---|
| `column_id` | `todo,doing` | coluna |
| `priority` | `high,urgent` | prioridade |
| `label_ids` | `<id>,<id>` | com `label_mode=all` (padrão) exige todas; `label_mode=any`, qualquer uma |
| `assignee_id` | `<id>`, `me` ou `none` | responsável; `none` = sem responsável |
| `created_by` | `<id>` ou `me` | criador |
| `status` | `open`, `completed` ou `all` | concluído ou não |
| `due_before` / `due_after` | `2026-12-01` | só a data: `due_before` vai até 23:59:59Z, `due_after` começa 00:00:00Z |
| `overdue` | `true` | atrasado: data no passado e não concluído |
| `has_due_date` | `true` / `false` | tem (ou não) data de entrega |
| `q` | `login` | busca por texto |
| `sort` | `position`, `created_at`, `updated_at`, `due_date`, `priority`, `title` | padrão `position` (com `q`, `updated_at`) |
| `order` | `asc` ou `desc` | padrão `asc` |

Exemplo: `GET /card/board/<id>?priority=urgent&assignee_id=me&status=open&sort=due_date`. Com `sort=due_date`, os cards sem data vêm por último nos dois sentidos. `sort=priority&order=desc` põe a maior prioridade primeiro. Valores inválidos devolvem 400 com a lista do que é aceito.

**Outras listagens:** `GET /project/?status=active&workspace_id=<id>&q=`, `GET /board/project/:id?q=`, `GET /project/member/:id?role=admin,manager`, `GET /workspace/member/:id?role=`, `GET /comment/card/:id?author_id=<id>|me`, `GET /attachment/card/:id?kind=file|link`.


## Rodando com Docker

Sobe a API e um MongoDB local com um comando, sem depender do Atlas.

```bash
cp .env.docker.example .env.docker      # troque o JWT_SECRET (mínimo 32 caracteres)
docker compose up --build -d
curl localhost:8000/health
docker compose logs -f api
docker compose down                     # mantém os dados
docker compose down -v                  # apaga os dados do Mongo
```

- `GET /health` é público e faz ping no Mongo (timeout de 2s): `200 {"status":"ok"}` ou `503` se o banco estiver fora. O healthcheck do container usa o próprio binário (`/app/api -healthcheck`), porque a imagem final (distroless) não tem `curl`.
- A imagem roda como usuário não-root e não contém o código-fonte nem nenhum `.env` (o `.env.docker` fica fora do git e do build, via `.gitignore` e `.dockerignore`). Os anexos ficam no volume `uploads` (`/app/uploads`).
- `docker stop` / `docker compose stop` mandam SIGTERM: a API termina as requests em andamento (até 10s), fecha a conexão com o Mongo e sai sem erro.
- Opcional, ver os dados no navegador (`http://localhost:8081`, só na máquina local): `docker compose --profile tools up -d`.
- Com `make` (Linux/macOS): `make up`, `make down`, `make logs`, `make test`, `make test-integration`. No Windows, use os comandos acima.

### Testes de integração contra o Mongo do compose

```bash
docker compose up -d mongo
MONGODB_URI=mongodb://localhost:27017 DATABASE_NAME=kanban_management TEST_DATABASE_NAME=kanban_management_test JWT_SECRET=<32+ caracteres> go test ./internal/tests/... -v
```

No PowerShell: defina as variáveis com `$env:MONGODB_URI = "mongodb://localhost:27017"` (e as demais) antes do `go test`.

> Estes arquivos foram escritos e revisados, mas ainda **não foram executados** (build da imagem, `docker compose up`): o Docker não estava instalado na máquina onde foram criados.
