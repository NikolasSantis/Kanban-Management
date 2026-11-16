# syntax=docker/dockerfile:1

# ---------- build ----------
# A versão deve ser >= a do go.mod (go 1.26.1). Se a tag 1.26 não existir no
# Docker Hub, use a estável mais próxima: docker build --build-arg GO_VERSION=1.26.1 .
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-alpine AS build

WORKDIR /src

# Dependências primeiro: esta camada só é refeita quando go.mod/go.sum mudam.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# Pasta de anexos (driver de storage local), já com o dono do usuário não-root.
RUN mkdir -p /out/uploads

# ---------- final ----------
# Sem shell, sem código-fonte, usuário não-root (uid 65532).
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/api /app/api
COPY --from=build --chown=nonroot:nonroot /out/uploads /app/uploads

ENV PORT=8000 \
    STORAGE_LOCAL_PATH=/app/uploads

EXPOSE 8000

USER nonroot:nonroot

# Distroless não tem curl: o próprio binário consulta GET /health.
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/api", "-healthcheck"]

ENTRYPOINT ["/app/api"]
