package beeep

import (
	"github.com/gen2brain/beeep"
	"github.com/oak3/github-notifier/application"
)

// BeeepNotificationService implements application.NotificationService using beeep
type BeeepNotificationService struct{}

// NewBeeepNotificationService creates a new beeep notification service
func NewBeeepNotificationService() application.NotificationService {
	return &BeeepNotificationService{}
}

// Notify sends a notification using beeep
func (s *BeeepNotificationService) Notify(title, message string) error {
	return beeep.Notify(title, message, "")
}
