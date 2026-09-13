package project

import (
	"kanban-management/internal/database"
	"kanban-management/internal/events"
	projectEvent "kanban-management/internal/events/project"
	"kanban-management/internal/models"
	"time"
)

func HandleProjectCreated(event events.Event) error {
	e := event.(*projectEvent.ProjectCreatedEvent)

	collection := database.GetCollection("project_members")

	now := time.Now()
	role := "admin"

	member := models.ProjectMember{
		UserID:    &e.UserID,
		ProjectID: &e.ProjectID,
		Role:      &role,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := collection.InsertOne(e.Ctx, member)

	if err != nil {
		return err
	}

	return nil
}
