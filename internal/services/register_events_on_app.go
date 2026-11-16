package services

import (
	"kanban-management/internal/events"
	project "kanban-management/internal/listeners/project_members"
	"kanban-management/internal/listeners/workspace"
	cardListeners "kanban-management/internal/listeners/cards"
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

	Dispatcher.Register(
		"project.deleted",
		project.HandleProjectDeleted,
	)

	Dispatcher.Register(
		"board.deleted",
		cardListeners.HandleBoardDeleted,
	)
}
