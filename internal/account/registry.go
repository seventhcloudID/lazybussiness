package account

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"threads-dashboard/internal/ai"
	"threads-dashboard/internal/buffer"
	"threads-dashboard/internal/instagram"
	"threads-dashboard/internal/lazy"
	"threads-dashboard/internal/schedule"
	"threads-dashboard/internal/threads"
)

const (
	accountsFile   = "accounts.json"
	accountsSubdir = "accounts"
)

type Meta struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	ThreadsUsername   string `json:"threads_username,omitempty"`
	InstagramUsername string `json:"instagram_username,omitempty"`
	TikTokUsername    string `json:"tiktok_username,omitempty"`
	ReplizThreadsID   string `json:"repliz_threads_id,omitempty"`
	ReplizInstagramID string `json:"repliz_instagram_id,omitempty"`
	ReplizTikTokID    string `json:"repliz_tiktok_id,omitempty"`
	CreatedAt         string `json:"created_at"`
}

type fileShape struct {
	ActiveID string `json:"active_id"`
	Accounts []Meta `json:"accounts"`
}

// Workspace holds per-account clients and stores.
type Workspace struct {
	Meta     Meta
	Dir      string
	Threads  *threads.Client
	IG       *instagram.Client
	Buffer   *buffer.Client
	Memory   *ai.MemoryStore
	Lazy     *lazy.Store
	Schedule *schedule.Store
	Deps     *lazy.Deps
	Sched    *lazy.Scheduler
	ThumbDir string
}

// Shared deps used by every workspace.
type Shared struct {
	AI        *ai.Client
	Thumb     *ai.ThumbnailClient
	Public    string
	Publisher lazy.Publisher
}

type Registry struct {
	mu         sync.RWMutex
	root       string
	filePath   string
	activeID   string
	order      []string
	meta       map[string]Meta
	workspaces map[string]*Workspace
	shared     Shared
}

// Open opens brand accounts under the default legacy path (.data).
// Prefer OpenAt with an org workspace directory.
func Open(shared Shared) (*Registry, error) {
	return OpenAt(".data", shared)
}

// OpenAt opens brand-account registry rooted at workspaceDir
// (contains accounts.json + accounts/).
func OpenAt(workspaceDir string, shared Shared) (*Registry, error) {
	root := strings.TrimSpace(workspaceDir)
	if root == "" {
		root = ".data"
	}
	r := &Registry{
		root:       root,
		filePath:   filepath.Join(root, accountsFile),
		meta:       map[string]Meta{},
		workspaces: map[string]*Workspace{},
		shared:     shared,
	}
	if err := os.MkdirAll(filepath.Join(root, accountsSubdir), 0o700); err != nil {
		return nil, err
	}
	if err := r.migrateIfNeeded(); err != nil {
		return nil, err
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	if r.activeID == "" && len(r.order) > 0 {
		r.activeID = r.order[0]
		_ = r.persist()
	}
	r.migrateLegacyBufferKey()
	for _, id := range r.order {
		if _, err := r.ensureWorkspace(id); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// migrateLegacyBufferKey memindah .data/buffer_key.json → akun aktif (sekali).
func (r *Registry) migrateLegacyBufferKey() {
	if r.activeID == "" {
		return
	}
	dst := filepath.Join(r.root, accountsSubdir, r.activeID, "buffer_key.json")
	if buffer.HasStoredAPIKey(dst) {
		// sudah ada key per-akun; hapus legacy kalau masih ada
		_ = os.Remove(buffer.LegacyKeyPath)
		return
	}
	if _, err := os.Stat(buffer.LegacyKeyPath); err == nil {
		moveIfExists(buffer.LegacyKeyPath, dst)
		log.Printf("accounts: Buffer key legacy → .data/accounts/%s/buffer_key.json", r.activeID)
		return
	}
	// Seed dari .env hanya ke akun aktif (bukan semua akun).
	buffer.SeedFromEnv(dst)
}

func (r *Registry) migrateIfNeeded() error {
	if _, err := os.Stat(r.filePath); err == nil {
		return nil
	}
	// Already have account dirs but missing index — rebuild
	accRoot := filepath.Join(r.root, accountsSubdir)
	entries, _ := os.ReadDir(accRoot)
	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	if len(ids) == 0 {
		id := "main"
		dir := filepath.Join(accRoot, id)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		moveIfExists(filepath.Join(r.root, "access_token"), filepath.Join(dir, "access_token"))
		moveIfExists(filepath.Join(r.root, "ig_access_token"), filepath.Join(dir, "ig_access_token"))
		moveIfExists(filepath.Join(r.root, "ai_memory.json"), filepath.Join(dir, "ai_memory.json"))
		moveIfExists(filepath.Join(r.root, "lazy_config.json"), filepath.Join(dir, "lazy_config.json"))
		moveIfExists(filepath.Join(r.root, "lazy_jobs.json"), filepath.Join(dir, "lazy_jobs.json"))
		moveIfExists(filepath.Join(r.root, "buffer_key.json"), filepath.Join(dir, "buffer_key.json"))
		moveDirIfExists(filepath.Join(r.root, "lazy-media"), filepath.Join(dir, "lazy-media"))
		moveDirIfExists(filepath.Join(r.root, "thumbs"), filepath.Join(dir, "thumbs"))
		ids = []string{id}
		log.Printf("accounts: migrasi data lama → .data/accounts/%s", id)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	accounts := make([]Meta, 0, len(ids))
	for _, id := range ids {
		accounts = append(accounts, Meta{
			ID: id, Name: id, CreatedAt: now,
		})
	}
	shape := fileShape{ActiveID: ids[0], Accounts: accounts}
	b, err := json.MarshalIndent(shape, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, b, 0o600)
}

func moveIfExists(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0o700)
	if err := os.Rename(src, dst); err != nil {
		// cross-device fallback
		b, rerr := os.ReadFile(src)
		if rerr != nil {
			return
		}
		if werr := os.WriteFile(dst, b, 0o600); werr == nil {
			_ = os.Remove(src)
		}
	}
}

func moveDirIfExists(src, dst string) {
	if _, err := os.Stat(src); err != nil {
		return
	}
	if _, err := os.Stat(dst); err == nil {
		return
	}
	if err := os.Rename(src, dst); err != nil {
		_ = copyDir(src, dst)
	}
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		_ = os.MkdirAll(filepath.Dir(target), 0o755)
		return os.WriteFile(target, b, info.Mode())
	})
}

func (r *Registry) load() error {
	b, err := os.ReadFile(r.filePath)
	if err != nil {
		return err
	}
	var shape fileShape
	if err := json.Unmarshal(b, &shape); err != nil {
		return err
	}
	r.order = nil
	r.meta = map[string]Meta{}
	for _, m := range shape.Accounts {
		if m.ID == "" {
			continue
		}
		r.meta[m.ID] = m
		r.order = append(r.order, m.ID)
	}
	r.activeID = shape.ActiveID
	if r.activeID != "" && r.meta[r.activeID].ID == "" && len(r.order) > 0 {
		r.activeID = r.order[0]
	}
	return nil
}

func (r *Registry) persist() error {
	r.mu.RLock()
	shape := fileShape{ActiveID: r.activeID}
	for _, id := range r.order {
		shape.Accounts = append(shape.Accounts, r.meta[id])
	}
	r.mu.RUnlock()
	b, err := json.MarshalIndent(shape, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, b, 0o600)
}

func (r *Registry) ensureWorkspace(id string) (*Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ws, ok := r.workspaces[id]; ok {
		return ws, nil
	}
	m, ok := r.meta[id]
	if !ok {
		return nil, fmt.Errorf("akun %q tidak ada", id)
	}
	dir := filepath.Join(r.root, accountsSubdir, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	thumbDir := filepath.Join(dir, "thumbs")
	_ = os.MkdirAll(thumbDir, 0o755)

	th := threads.NewWithTokenPath(filepath.Join(dir, "access_token"))
	ig := instagram.NewWithTokenPath(filepath.Join(dir, "ig_access_token"))
	buf := buffer.NewAt(filepath.Join(dir, "buffer_key.json"))
	mem := ai.NewMemoryStoreAt(filepath.Join(dir, "ai_memory.json"))
	lz := lazy.NewStoreAt(
		filepath.Join(dir, "lazy_config.json"),
		filepath.Join(dir, "lazy_jobs.json"),
		filepath.Join(dir, "lazy-media"),
	)
	schedStore := schedule.NewStoreAt(filepath.Join(dir, "scheduled_posts.json"))
	deps := &lazy.Deps{
		Store:     lz,
		Threads:   th,
		IG:        ig,
		AI:        r.shared.AI,
		Thumb:     r.shared.Thumb,
		Buffer:    buf,
		Memory:    mem,
		Public:    r.shared.Public,
		ThumbDir:  thumbDir,
		Schedule:  schedStore,
		Publisher: r.shared.Publisher,
		ResolveAccountID: func(platform string) string {
			return r.PlatformAccountID(id, platform)
		},
	}
	sched := lazy.NewScheduler(deps)
	ws := &Workspace{
		Meta: m, Dir: dir,
		Threads: th, IG: ig, Buffer: buf, Memory: mem, Lazy: lz, Schedule: schedStore,
		Deps: deps, Sched: sched, ThumbDir: thumbDir,
	}
	r.workspaces[id] = ws
	return ws, nil
}

func (r *Registry) StartSchedulers() {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, id := range r.order {
		ws := r.workspaces[id]
		if ws == nil || ws.Sched == nil {
			continue
		}
		ws.Sched.Start()
		log.Printf("lazy scheduler: akun %s", id)
	}
}

func (r *Registry) StopSchedulers() {
	if r == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, id := range r.order {
		ws := r.workspaces[id]
		if ws == nil || ws.Sched == nil {
			continue
		}
		ws.Sched.Stop()
	}
}

func (r *Registry) Active() *Workspace {
	r.mu.RLock()
	id := r.activeID
	r.mu.RUnlock()
	ws, err := r.ensureWorkspace(id)
	if err != nil {
		// should not happen after Open
		return nil
	}
	return ws
}

func (r *Registry) ActiveID() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.activeID
}

func (r *Registry) List() []Meta {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Meta, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.meta[id])
	}
	return out
}

func (r *Registry) All() []*Workspace {
	r.mu.RLock()
	ids := append([]string{}, r.order...)
	r.mu.RUnlock()
	out := make([]*Workspace, 0, len(ids))
	for _, id := range ids {
		ws, err := r.ensureWorkspace(id)
		if err != nil {
			continue
		}
		out = append(out, ws)
	}
	return out
}

func (r *Registry) Get(id string) (*Workspace, error) {
	return r.ensureWorkspace(id)
}

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimPrefix(s, "@")
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = fmt.Sprintf("akun-%d", time.Now().Unix()%100000)
	}
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

func (r *Registry) Create(name string) (*Workspace, error) {
	base := slugify(name)
	r.mu.Lock()
	id := base
	for i := 2; r.meta[id].ID != ""; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}
	m := Meta{
		ID:        id,
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if m.Name == "" {
		m.Name = id
	}
	r.meta[id] = m
	r.order = append(r.order, id)
	if r.activeID == "" {
		r.activeID = id
	}
	r.mu.Unlock()
	if err := r.persist(); err != nil {
		return nil, err
	}
	ws, err := r.ensureWorkspace(id)
	if err != nil {
		return nil, err
	}
	ws.Sched.Start()
	return ws, nil
}

func (r *Registry) Switch(id string) error {
	r.mu.RLock()
	_, ok := r.meta[id]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("akun %q tidak ada", id)
	}
	if _, err := r.ensureWorkspace(id); err != nil {
		return err
	}
	r.mu.Lock()
	r.activeID = id
	r.mu.Unlock()
	return r.persist()
}

func (r *Registry) UpdateMeta(id string, mutate func(*Meta)) error {
	r.mu.Lock()
	m, ok := r.meta[id]
	if !ok {
		r.mu.Unlock()
		return fmt.Errorf("akun %q tidak ada", id)
	}
	mutate(&m)
	r.meta[id] = m
	if ws := r.workspaces[id]; ws != nil {
		ws.Meta = m
	}
	r.mu.Unlock()
	return r.persist()
}

// PlatformAccountID returns the Repliz account bound to a local brand workspace.
func (r *Registry) PlatformAccountID(workspaceID, platform string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m := r.meta[workspaceID]
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "threads":
		return strings.TrimSpace(m.ReplizThreadsID)
	case "instagram", "ig":
		return strings.TrimSpace(m.ReplizInstagramID)
	case "tiktok":
		return strings.TrimSpace(m.ReplizTikTokID)
	default:
		return ""
	}
}

func (r *Registry) Delete(id string) error {
	r.mu.Lock()
	if len(r.order) <= 1 {
		r.mu.Unlock()
		return fmt.Errorf("minimal satu akun harus ada")
	}
	if _, ok := r.meta[id]; !ok {
		r.mu.Unlock()
		return fmt.Errorf("akun %q tidak ada", id)
	}
	delete(r.meta, id)
	next := make([]string, 0, len(r.order)-1)
	for _, x := range r.order {
		if x != id {
			next = append(next, x)
		}
	}
	r.order = next
	if r.activeID == id {
		r.activeID = next[0]
	}
	ws := r.workspaces[id]
	delete(r.workspaces, id)
	r.mu.Unlock()
	if ws != nil && ws.Sched != nil {
		ws.Sched.Stop()
	}
	_ = r.persist()
	// keep files on disk for safety — user can remove folder manually
	return nil
}

// FindLazyMedia returns absolute path for a relative lazy media file across accounts.
func (r *Registry) FindLazyMedia(rel string) string {
	rel = filepath.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "." || strings.Contains(rel, "..") {
		return ""
	}
	for _, ws := range r.All() {
		p := filepath.Join(ws.Lazy.MediaDir(), rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func (r *Registry) FindThumbMedia(rel string) string {
	rel = filepath.Clean("/" + rel)
	rel = strings.TrimPrefix(rel, string(filepath.Separator))
	if rel == "." || strings.Contains(rel, "..") {
		return ""
	}
	for _, ws := range r.All() {
		p := filepath.Join(ws.ThumbDir, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	// workspace-level legacy
	p := filepath.Join(r.root, "thumbs", rel)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	// global legacy (.data/thumbs) — API lama pernah simpan di sini
	p = filepath.Join(".data", "thumbs", rel)
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}
