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
