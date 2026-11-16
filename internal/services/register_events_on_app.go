package services

import (
	"kanban-management/internal/events"
	boardListeners "kanban-management/internal/listeners/boards"
	cardListeners "kanban-management/internal/listeners/cards"
	commentListeners "kanban-management/internal/listeners/comments"
	labelListeners "kanban-management/internal/listeners/labels"
	project "kanban-management/internal/listeners/project_members"
	projectListeners "kanban-management/internal/listeners/projects"
	"kanban-management/internal/listeners/workspace"
)

var Dispatcher *events.Dispatcher

func RegisterEvents() {

	Dispatcher = events.NewDispatcher()

	Dispatcher.Register(
		"user.deleted",
		workspace.HandleUserDeleted,
	)

	// workspace.deleted → membros do workspace e, em cascata, os projetos
	// (cada um dispara project.deleted).
	Dispatcher.Register(
		"workspace.deleted",
		workspace.HandleWorkspaceDeleted,
	)

	Dispatcher.Register(
		"workspace.deleted",
		projectListeners.CascadeWorkspaceDeleted(Dispatcher),
	)

	Dispatcher.Register(
		"project.created",
		project.HandleProjectCreated,
	)

	// project.deleted → membros do projeto e, em cascata, os boards
	// (cada um dispara board.deleted).
	Dispatcher.Register(
		"project.deleted",
		project.HandleProjectDeleted,
	)

	Dispatcher.Register(
		"project.deleted",
		boardListeners.CascadeProjectDeleted(Dispatcher),
	)

	// board.deleted → cards e labels do board.
	Dispatcher.Register(
		"board.deleted",
		cardListeners.HandleBoardDeleted,
	)

	Dispatcher.Register(
		"board.deleted",
		labelListeners.HandleBoardDeleted,
	)

	// card.deleted → comentários do card (anexos entram na etapa 11).
	Dispatcher.Register(
		"card.deleted",
		commentListeners.HandleCardDeleted,
	)

	// card.deleted → anexos do card (os arquivos físicos ficam no storage).
	Dispatcher.Register(
		"card.deleted",
		commentListeners.HandleCardDeletedAttachments,
	)

	// label.deleted → remove a label dos cards.
	Dispatcher.Register(
		"label.deleted",
		cardListeners.HandleLabelDeleted,
	)
}
