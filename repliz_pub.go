package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"threads-dashboard/internal/repliz"
	"threads-dashboard/internal/schedule"
)

type replizPub struct{}

func replizAccountLive(a repliz.Account) bool {
	return a.AccountID() != "" && (a.IsConnected || strings.TrimSpace(a.Username) != "")
}

func (replizPub) ThreadsOK(accountID string) bool {
	acc, err := replizAccountForID(context.Background(), accountID, "threads")
	return err == nil && replizAccountLive(acc)
}

func (replizPub) InstagramOK(accountID string) bool {
	acc, err := replizAccountForID(context.Background(), accountID, "instagram")
	return err == nil && replizAccountLive(acc)
}

func (replizPub) TikTokOK(accountID string) bool {
	acc, err := replizAccountForID(context.Background(), accountID, "tiktok")
	return err == nil && replizAccountLive(acc)
}

func (replizPub) PublishThreads(accountID string, parts []string, imageURL, videoURL string) ([]string, error) {
	return publishReplizPartsTo(context.Background(), accountID, "threads", parts, imageURL, videoURL, time.Now().Add(25*time.Second))
}

func (replizPub) PublishIGCarousel(accountID string, urls []string, caption string) (string, error) {
	return publishReplizCarousel(context.Background(), accountID, "instagram", urls, caption)
}

func (replizPub) PublishTikTokCarousel(accountID string, urls []string, caption string) (string, error) {
	return publishReplizCarousel(context.Background(), accountID, "tiktok", urls, caption)
}

func publishReplizCarousel(ctx context.Context, accountID, platform string, urls []string, caption string) (string, error) {
	acc, err := replizAccountForID(ctx, accountID, platform)
	if err != nil {
		return "", err
	}
	if !replizAccountLive(acc) {
		return "", fmt.Errorf("akun Repliz %s belum terhubung", platform)
	}
	medias := make([]repliz.Media, 0, len(urls))
	for _, u := range urls {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		medias = append(medias, repliz.Media{Type: "image", URL: u, Thumbnail: u})
	}
	if len(medias) < 2 {
		return "", fmt.Errorf("carousel %s butuh minimal 2 gambar", platform)
	}
	req := repliz.ScheduleReq{
		Title:       firstLine(caption, 80),
		Description: caption,
		Type:        "album",
		Medias:      medias,
		AccountID:   acc.AccountID(),
		ScheduleAt:  time.Now().UTC().Add(25 * time.Second).Format("2006-01-02T15:04:05.000Z"),
	}
	if strings.EqualFold(platform, "tiktok") {
		req.AdditionalInfo = repliz.TikTokAdditionalInfo(tiktokReplizDraft(), true)
	}
	id, err := replizCli.CreateSchedule(ctx, req)
	return id, err
}

func tiktokReplizDraft() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("LAZY_TIKTOK_DRAFT")))
	if v == "0" || v == "false" || v == "no" {
		return false
	}
	return true // default: draft ke inbox TikTok
}

func replizAccountForID(ctx context.Context, accountID, platform string) (repliz.Account, error) {
	if replizCli == nil || !replizCli.Ready() {
		return repliz.Account{}, fmt.Errorf("Repliz belum disambungkan")
	}
	accountID = strings.TrimSpace(accountID)
	platform = strings.ToLower(strings.TrimSpace(platform))
	if accountID == "" {
		return repliz.Account{}, fmt.Errorf("akun %s belum dipilih di workspace", platform)
	}
	acc, err := replizCli.GetAccount(ctx, accountID)
	if err != nil {
		return repliz.Account{}, err
	}
	if !strings.EqualFold(acc.Type, platform) {
		return repliz.Account{}, fmt.Errorf("akun %s tidak cocok untuk kanal %s", acc.Type, platform)
	}
	return acc, nil
}

func (replizPub) PublishScheduled(p schedule.Post) ([]string, error) {
	if err := schedule.ValidateParts(p); err != nil {
		return nil, err
	}
	parts := schedule.PartsOf(p)
	at := p.RunAt
	if at.IsZero() || at.Before(time.Now().Add(15*time.Second)) {
		at = time.Now().Add(25 * time.Second)
	}
	if rid := strings.TrimSpace(p.ReplyToID); rid != "" {
		acc, err := pickReplizByType(context.Background(), "", "threads")
		if err != nil {
			return nil, err
		}
		id, err := replizCli.CreateComment(context.Background(), rid, acc.AccountID(), parts[0], "")
		if err != nil {
			return nil, err
		}
		return []string{id}, nil
	}
	return publishReplizParts(context.Background(), "threads", parts, p.ImageURL, p.VideoURL, at)
}

func publishReplizParts(ctx context.Context, platform string, parts []string, imageURL, videoURL string, at time.Time) ([]string, error) {
	acc, err := pickReplizByType(ctx, "", platform)
	if err != nil {
		return nil, err
	}
	return publishReplizPartsTo(ctx, acc.AccountID(), platform, parts, imageURL, videoURL, at)
}

func publishReplizPartsTo(ctx context.Context, accountID, platform string, parts []string, imageURL, videoURL string, at time.Time) ([]string, error) {
	if replizCli == nil || !replizCli.Ready() {
		return nil, fmt.Errorf("Repliz belum disambungkan — set REPLIZ_ACCESS_KEY")
	}
	cleaned := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			cleaned = append(cleaned, p)
		}
	}
	if len(cleaned) == 0 {
		return nil, fmt.Errorf("konten kosong")
	}
	acc, err := replizAccountForID(ctx, accountID, platform)
	if err != nil {
		return nil, err
	}
	if !replizAccountLive(acc) {
		return nil, fmt.Errorf("akun Repliz %s belum terhubung — sambungkan di /app/akun", platform)
	}
	kind := repliz.MediaTypeFromKind("", imageURL, videoURL)
	medias := []repliz.Media{}
	if kind == "image" && strings.TrimSpace(imageURL) != "" {
		medias = []repliz.Media{{Type: "image", URL: imageURL, Thumbnail: imageURL}}
	}
	if kind == "video" && strings.TrimSpace(videoURL) != "" {
		medias = []repliz.Media{{Type: "video", URL: videoURL}}
	}
	id, err := replizCli.CreateSchedule(ctx, repliz.ScheduleReq{
		Title:       firstLine(cleaned[0], 80),
		Description: cleaned[0],
		Type:        kind,
		Medias:      medias,
		AccountID:   acc.AccountID(),
		ScheduleAt:  at.UTC().Format("2006-01-02T15:04:05.000Z"),
		Replies:     repliz.ReplyParts(cleaned),
	})
	if err != nil {
		return nil, err
	}
	return []string{id}, nil
}

func writeGoneMeta(w http.ResponseWriter, msg string) {
	if strings.TrimSpace(msg) == "" {
		msg = "Token Meta tidak dipakai. Hubungkan akun lewat Repliz di /app/akun."
	}
	writeErr(w, http.StatusGone, msg)
}

func replizConnectedPair(ctx context.Context) (thAcc, igAcc repliz.Account, thOK, igOK bool) {
	if replizCli == nil || !replizCli.Ready() {
		return
	}
	thAcc, err := pickReplizByType(ctx, "", "threads")
	if err == nil && replizAccountLive(thAcc) {
		thOK = true
	}
	igAcc, err = pickReplizByType(ctx, "", "instagram")
	if err == nil && replizAccountLive(igAcc) {
		igOK = true
	}
	return
}
