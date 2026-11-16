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
