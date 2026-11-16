// Migração idempotente que garante que o dono de cada recurso seja admin:
//
//  1. workspaces não deletados: o membro cujo user_id é o owner_id vira admin.
//  2. projetos não deletados: o dono ganha um registro admin em project_members
//     (inserido se não existir, promovido se tiver outra role).
//
// Por padrão só conta (dry-run); use -apply para gravar.
//
//	go run ./cmd/migrate-owners-admin          # só conta
//	go run ./cmd/migrate-owners-admin -apply   # grava
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

	workspacesChanged := migrateWorkspaces(ctx, *apply)
	projectsInserted, projectsPromoted := migrateProjects(ctx, *apply)

	mode := "dry-run (nada gravado)"
	if *apply {
		mode = "aplicado"
	}

	fmt.Printf("workspaces: donos promovidos a admin: %d\n", workspacesChanged)
	fmt.Printf("projetos: donos inseridos como admin: %d | donos promovidos a admin: %d\n", projectsInserted, projectsPromoted)
	fmt.Println(mode)
}

func migrateWorkspaces(ctx context.Context, apply bool) int64 {
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

		if !apply {
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

	return changed
}

func migrateProjects(ctx context.Context, apply bool) (inserted, promoted int64) {
	cursor, err := database.GetCollection("projects").Find(ctx, bson.M{"$or": services.NotDeleted()})
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(ctx)

	var projects []models.Project
	if err := cursor.All(ctx, &projects); err != nil {
		log.Fatal(err)
	}

	members := database.GetCollection("project_members")

	for _, project := range projects {
		base := bson.M{
			"project_id": project.ID,
			"user_id":    project.Owner_id,
			"$or":        services.NotDeleted(),
		}

		total, err := members.CountDocuments(ctx, base)
		if err != nil {
			log.Fatal(err)
		}

		if total == 0 {
			inserted++

			if apply {
				now := time.Now()
				projectID, ownerID, role := project.ID, project.Owner_id, services.RoleAdmin

				_, err := members.InsertOne(ctx, models.ProjectMember{
					UserID:    &ownerID,
					ProjectID: &projectID,
					Role:      &role,
					CreatedAt: now,
					UpdatedAt: now,
				})
				if err != nil {
					log.Fatal(err)
				}
			}

			continue
		}

		notAdmin := bson.M{
			"project_id": project.ID,
			"user_id":    project.Owner_id,
			"role":       bson.M{"$ne": services.RoleAdmin},
			"$or":        services.NotDeleted(),
		}

		if !apply {
			count, err := members.CountDocuments(ctx, notAdmin)
			if err != nil {
				log.Fatal(err)
			}
			promoted += count
			continue
		}

		result, err := members.UpdateMany(ctx, notAdmin, bson.M{
			"$set": bson.M{"role": services.RoleAdmin, "updated_at": time.Now()},
		})
		if err != nil {
			log.Fatal(err)
		}
		promoted += result.ModifiedCount
	}

	return inserted, promoted
}
