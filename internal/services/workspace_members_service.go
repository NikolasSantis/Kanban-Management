package services

import (
	"context"
	"kanban-management/internal/database"
	"kanban-management/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var UserRoles = []string{
	"member",
	"manager",
	"admin",
}

var UserRolesCanUpdate = []string{
	"manager",
	"admin",
}

func GetWorkspaceUserRole(userID bson.ObjectID, workspaceID bson.ObjectID) (string, error) {
	var workspaceMember models.WorkspaceMember

	collection := database.GetCollection("workspaces_members")
	ctx := context.Background()

	err := collection.FindOne(ctx, bson.M{
		"user_id":      userID,
		"workspace_id": workspaceID,
	}).Decode(&workspaceMember)

	return workspaceMember.Role, err
}

func AddOwnerProjectAsOneMember(userID bson.ObjectID, workspaceID bson.ObjectID) bool {
	now := time.Now()
	collection := database.GetCollection("workspaces_members")
	ctx := context.Background()

	_, err := collection.InsertOne(ctx, bson.M{
		"user_id":      userID,
		"workspace_id": workspaceID,
		"role":         "manager",
		"created_at":   now,
		"updated_at":   now,
	})

	if err != nil {
		return false
	}

	return true
}
