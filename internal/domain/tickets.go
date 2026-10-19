package domain

import (
	"context"
	"time"
)

// Ticket is one numbered exam ticket ("bilet") and what the learner has
// scored on it. BestCorrect is the best completed attempt, as for topic tests.
type Ticket struct {
	Number        int        `json:"number"`
	QuestionCount int        `json:"questionCount"`
	Attempts      int        `json:"attempts"`
	BestCorrect   *int       `json:"bestCorrect,omitempty"`
	LastAttemptAt *time.Time `json:"lastAttemptAt,omitempty"`
}

// TicketRepository stores tickets as ordered lists of questions. Only
// Postgres holds them (cmd/import-tickets fills them); without it the app
// simply has no tickets.
type TicketRepository interface {
	// ListTickets returns every ticket of a pack in number order, with the
	// learner's attempts and best score on each.
	ListTickets(ctx context.Context, userID int64, packID string) ([]Ticket, error)
	// GetTicketQuestions returns one ticket's questions in ticket order, or
	// none when the ticket does not exist.
	GetTicketQuestions(ctx context.Context, packID string, number int) ([]*Question, error)
}
