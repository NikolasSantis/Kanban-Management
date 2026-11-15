// Migração idempotente: promove a admin o membro que é dono (owner_id) de cada
// workspace não deletado. Por padrão só conta (dry-run); use -apply para gravar.
//
//	go run ./cmd/migrate-workspace-owner-admin          # só conta
//	go run ./cmd/migrate-workspace-owner-admin -apply   # grava
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"kanban-management/config"
	"kanban-management/internal/database"
	"kanban-management/internal/models"
	"kanban-management/internal/services"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func main() {
	apply := flag.Bool("apply", false, "grava as alterações (sem a flag, apenas conta)")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config inválida: %v", err)
	}

	if err := database.ConnectDB(cfg); err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	cursor, err := database.GetCollection("workspaces").Find(ctx, bson.M{"$or": services.NotDeleted()})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var workspaces []models.Workspace
	if err := cursor.All(ctx, &workspaces); err != nil {
		log.Fatal(err)
	}

	members := database.GetCollection("workspaces_members")
	changed := int64(0)

	for _, workspace := range workspaces {
		filter := bson.M{
			"workspace_id": workspace.ID,
			"user_id":      workspace.OwnderID,
			"role":         bson.M{"$ne": services.RoleAdmin},
			"$or":          services.NotDeleted(),
		}

		if !*apply {
			count, err := members.CountDocuments(ctx, filter)
			if err != nil {
				log.Fatal(err)
			}
			changed += count
			continue
		}

		result, err := members.UpdateMany(ctx, filter, bson.M{
			"$set": bson.M{"role": services.RoleAdmin, "updated_at": time.Now()},
		})
		if err != nil {
			log.Fatal(err)
		}
		changed += result.ModifiedCount
	}

	mode := "dry-run (nada gravado)"
	if *apply {
		mode = "aplicado"
	}

	fmt.Printf("workspaces ativos: %d | membros-dono promovidos a admin: %d | %s\n", len(workspaces), changed, mode)
}
