package services

import (
	"kanban-management/internal/events"
	"kanban-management/internal/listeners/workspace"
)

var Dispatcher *events.Dispatcher

func RegisterEvents() {

	Dispatcher = events.NewDispatcher()

	Dispatcher.Register(
		"user.deleted",
		workspace.HandleUserDeleted,
	)
}
