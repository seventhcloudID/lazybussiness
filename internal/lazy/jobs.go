package lazy

import "strings"

// NormalizeJob sinkronkan field legacy buffer_* setelah Repliz TikTok terkirim.
// buffer_post_id saja (Buffer Notify Me lama) TIDAK dianggap schedule Repliz.
func NormalizeJob(j *Job) {
	if j == nil {
		return
	}
	if j.TikTokScheduleID != "" {
		j.BufferPostID = j.TikTokScheduleID
	}
	if j.TikTokError != "" {
		j.BufferError = j.TikTokError
	}
}

func (j Job) IGPublished() bool {
	return strings.TrimSpace(j.IGMediaID) != "" || strings.TrimSpace(j.IGContainer) != ""
}

func (j Job) TikTokPublished() bool {
	return strings.TrimSpace(j.TikTokScheduleID) != ""
}

func (j Job) HasLegacyBufferTikTok() bool {
	return !j.TikTokPublished() && strings.TrimSpace(j.BufferPostID) != ""
}

func (j Job) TikTokSchedule() string {
	return strings.TrimSpace(j.TikTokScheduleID)
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
