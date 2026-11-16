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
