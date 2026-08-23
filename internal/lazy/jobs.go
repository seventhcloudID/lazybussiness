package lazy

import "strings"

// NormalizeJob menyamakan field legacy (buffer_*) dengan field Repliz carousel.
func NormalizeJob(j *Job) {
	if j == nil {
		return
	}
	if j.TikTokScheduleID == "" {
		j.TikTokScheduleID = strings.TrimSpace(j.BufferPostID)
	}
	if j.TikTokError == "" {
		j.TikTokError = strings.TrimSpace(j.BufferError)
	}
	j.BufferPostID = j.TikTokScheduleID
	j.BufferError = j.TikTokError
}

func (j Job) IGPublished() bool {
	return strings.TrimSpace(j.IGMediaID) != "" || strings.TrimSpace(j.IGContainer) != ""
}

func (j Job) TikTokPublished() bool {
	id := strings.TrimSpace(j.TikTokScheduleID)
	if id == "" {
		id = strings.TrimSpace(j.BufferPostID)
	}
	return id != ""
}

func (j Job) TikTokSchedule() string {
	if id := strings.TrimSpace(j.TikTokScheduleID); id != "" {
		return id
	}
	return strings.TrimSpace(j.BufferPostID)
}

func jobHasCarouselContent(job Job) bool {
	if len(job.ImageURLs) >= 2 {
		return true
	}
	parts := job.Parts
	if len(parts) < 2 {
		parts = job.PrefilledParts
	}
	return len(parts) >= 2
}

func jobFinished(job Job) bool {
	return job.Status == StatusDone || job.Status == StatusSkippedIG
}

// JobCarouselReady true jika carousel siap dikirim ulang ke Repliz untuk kanal tertentu.
func JobCarouselReady(job Job, channel string) bool {
	if !jobFinished(job) {
		return false
	}
	channel = normalizeChannelName(channel)
	switch channel {
	case "instagram":
		return !job.IGPublished() && jobHasCarouselContent(job)
	case "tiktok":
		return !job.TikTokPublished() && jobHasCarouselContent(job)
	default:
		return false
	}
}

func normalizeChannelName(channel string) string {
	channel = strings.ToLower(strings.TrimSpace(channel))
	if channel == "ig" {
		return "instagram"
	}
	return channel
}
