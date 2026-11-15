package services

import (
	"context"
	"errors"

	"kanban-management/internal/database"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// ErrLastAdmin: a operação deixaria o projeto/workspace sem nenhum admin (→ 409).
var ErrLastAdmin = errors.New("must have at least one admin")

// CanChangeMemberRole diz se quem tem actorRole pode mudar um membro de
// currentRole para newRole (newRole vazio = remoção). Só admin adiciona,
// promove, rebaixa ou remove admin. A permissão de gerenciar membros em si é
// checada antes, via Can(..., Action*ManageMembers).
func CanChangeMemberRole(actorRole, currentRole, newRole string) bool {
	if currentRole == RoleAdmin || newRole == RoleAdmin {
		return actorRole == RoleAdmin
	}

	return true
}

// EnsureNotLastAdmin retorna ErrLastAdmin se targetRole é admin e ele é o
// único admin ativo do escopo. collection: "project_members" (scopeField
// "project_id") ou "workspaces_members" (scopeField "workspace_id").
// Só chamar quando a mudança tira o admin do membro (remoção ou rebaixamento).
func EnsureNotLastAdmin(ctx context.Context, collection, scopeField string, scopeID bson.ObjectID, targetRole string) error {
	if targetRole != RoleAdmin {
		return nil
	}

	count, err := database.GetCollection(collection).CountDocuments(ctx, bson.M{
		scopeField: scopeID,
		"role":     RoleAdmin,
		"$or":      NotDeleted(),
	})

	if err != nil {
		return err
	}

	if count <= 1 {
		return ErrLastAdmin
	}

	return nil
}
