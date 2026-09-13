package services

import (
	"kanban-management/internal/events"
	project "kanban-management/internal/listeners/project_members"
	"kanban-management/internal/listeners/workspace"
)

var Dispatcher *events.Dispatcher

func RegisterEvents() {

	Dispatcher = events.NewDispatcher()

	Dispatcher.Register(
		"user.deleted",
		workspace.HandleUserDeleted,
	)

	Dispatcher.Register(
		"workspace.deleted",
		workspace.HandleWorkspaceDeleted,
	)

	Dispatcher.Register(
		"project.created",
		project.HandleProjectCreated,
	)
}
