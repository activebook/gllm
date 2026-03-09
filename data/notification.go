package data

import "sync/atomic"

// Global channel for passing the async notification result.
var (
	notificationCh = make(chan string, 1)
	checkResolved  atomic.Bool
)

// StoreNotification safely queues a new notification string.
func StoreNotification(text string) {
	select {
	case notificationCh <- text:
		if text != "" {
			// new notification is available
			checkResolved.Store(false)
		}
	default:
		// notification channel is full, drop the new notification
	}
}

// ResolveNotification marks the notification as resolved.
func ResolveNotification() {
	checkResolved.Store(true)
}

// GetNotification populates a notification string if a newer notification is waiting.
// Returns (text, true) if the background check finished.
// Returns ("", false) if the check result is not yet available.
func GetNotification() (string, bool) {
	if checkResolved.Load() {
		return "", true
	}
	select {
	case text := <-notificationCh:
		checkResolved.Store(true)
		return text, true
	default:
		return "", false
	}
}
