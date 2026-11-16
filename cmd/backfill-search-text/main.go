// Preenche o campo search_text nos cards, boards e projetos criados antes da
// etapa 13. Idempotente: só toca em documentos que ainda não têm o campo.
// Por padrão só conta (dry-run); use -apply para gravar.
//
//	go run ./cmd/backfill-search-text          # só conta
//	go run ./cmd/backfill-search-text -apply   # grava
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/logger"
	"kanban-management/internal/search"
)

func main() {
	apply := flag.Bool("apply", false, "grava as alterações (sem a flag, apenas conta)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config inválida: %v", err)
	}

	cfg.LogLevel = "error"
	logger.Setup(cfg)

	if err := database.ConnectDB(cfg); err != nil {
		log.Fatal(err)
	}

	result, err := search.Backfill(context.Background(), *apply)
	if err != nil {
		log.Fatal(err)
	}

	mode := "dry-run (nada gravado)"
	if *apply {
		mode = "aplicado"
	}

	fmt.Printf("search_text a preencher: cards=%d boards=%d projects=%d | %s\n", result.Cards, result.Boards, result.Projects, mode)
}
