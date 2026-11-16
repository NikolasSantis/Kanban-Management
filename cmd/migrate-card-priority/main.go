// Migração idempotente: cards criados antes da etapa 07 ganham priority=medium.
// Por padrão só conta (dry-run); use -apply para gravar.
//
//	go run ./cmd/migrate-card-priority          # só conta
//	go run ./cmd/migrate-card-priority -apply   # grava
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/logger"
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

	ctx := context.Background()

	if !*apply {
		count, err := database.CountCardsWithoutPriority(ctx)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("cards sem prioridade que seriam alterados: %d | dry-run (nada gravado)\n", count)

		return
	}

	changed, err := database.MigrateCardPriorities(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("cards alterados: %d | aplicado\n", changed)
}
