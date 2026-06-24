package notifier

import "context"

// Notification is a message ready for delivery.
type Notification struct {
	ChatIDs   []int64
	Text      string
	ParseMode string
}

// Notifier sends notifications to a messaging backend.
type Notifier interface {
	Send(ctx context.Context, n Notification) error
}
