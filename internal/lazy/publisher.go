package lazy

import "threads-dashboard/internal/schedule"

// Publisher sends posts through Repliz (not Meta Graph).
type Publisher interface {
	ThreadsOK(accountID string) bool
	InstagramOK(accountID string) bool
	TikTokOK(accountID string) bool
	PublishThreads(accountID string, parts []string, imageURL, videoURL string) ([]string, error)
	PublishIGCarousel(accountID string, urls []string, caption string) (string, error)
	PublishTikTokCarousel(accountID string, urls []string, caption string) (string, error)
	PublishScheduled(p schedule.Post) ([]string, error)
}
