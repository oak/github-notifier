package application

// NotificationService defines the interface for sending notifications
type NotificationService interface {
	Notify(title, message, icon string) error
}
