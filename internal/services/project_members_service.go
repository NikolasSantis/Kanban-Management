package services

import (
	"context"
	"errors"

	"kanban-management/internal/database"
	"kanban-management/internal/models"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func GetProjectUserRole(userID, projectID bson.ObjectID) (string, error) {
	collection := database.GetCollection("project_members")

	ctx := context.Background()

	var member models.ProjectMember

	err := collection.FindOne(ctx, bson.M{
		"project_id": projectID,
		"user_id":    userID,
		"$or": []bson.M{
			{
				"deleted_at": bson.M{
					"$exists": false,
				},
			},
			{
				"deleted_at": nil,
			},
		},
	}).Decode(&member)

	if err != nil {
		return "", errors.New("user is not a project member")
	}

	if member.Role == nil {
		return "", errors.New("member role not found")
	}

	return *member.Role, nil
}
