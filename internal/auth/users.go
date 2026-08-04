package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleAdmin  = "admin"
	RoleTenant = "tenant"
)

type Billing struct {
	Status string `json:"status"` // trial | active | past_due | suspended
	Plan   string `json:"plan,omitempty"`
	Notes  string `json:"notes,omitempty"`
	DueAt  string `json:"due_at,omitempty"`
}

type User struct {
	ID           string  `json:"id"`
	Username     string  `json:"username"`
	PasswordHash string  `json:"password_hash"`
	Role         string  `json:"role"` // admin | tenant
	TenantID     string  `json:"tenant_id,omitempty"`
	Active       bool    `json:"active"`
	CreatedAt    string  `json:"created_at"`
	Billing      Billing `json:"billing,omitempty"` // denormalized note on user; tenant billing lives on tenant meta
}

type usersFile struct {
	Users []User `json:"users"`
}

type UserStore struct {
	mu   sync.Mutex
	path string
	list []User
}

func OpenUsers(dataRoot string) (*UserStore, error) {
	if dataRoot == "" {
		dataRoot = ".data"
	}
	s := &UserStore{path: filepath.Join(dataRoot, "users.json")}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *UserStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.list = nil
			return nil
		}
		return err
	}
	var f usersFile
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	s.list = f.Users
	return nil
}

func (s *UserStore) persistLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(usersFile{Users: s.list}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *UserStore) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.list)
}

func (s *UserStore) List() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]User, len(s.list))
	copy(out, s.list)
	for i := range out {
		out[i].PasswordHash = ""
	}
	return out
}

func (s *UserStore) FindByUsername(username string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username = strings.ToLower(strings.TrimSpace(username))
	for i := range s.list {
		if strings.ToLower(s.list[i].Username) == username {
			u := s.list[i]
			return &u, true
		}
	}
	return nil, false
}

func (s *UserStore) FindByID(id string) (*User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID == id {
			u := s.list[i]
			return &u, true
		}
	}
	return nil, false
}

func HashPassword(pass string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, pass string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil
}

// SeedAdminFromEnv creates an admin user from AUTH_USER/AUTH_PASSWORD if store empty.
func (s *UserStore) SeedAdminFromEnv() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.list) > 0 {
		return false, nil
	}
	user := strings.TrimSpace(os.Getenv("AUTH_USER"))
	pass := strings.TrimSpace(os.Getenv("AUTH_PASSWORD"))
	if user == "" || pass == "" {
		return false, nil
	}
	hash, err := HashPassword(pass)
	if err != nil {
		return false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	s.list = []User{{
		ID:           "admin",
		Username:     user,
		PasswordHash: hash,
		Role:         RoleAdmin,
		Active:       true,
		CreatedAt:    now,
		Billing:      Billing{Status: "active", Plan: "admin"},
	}}
	if err := s.persistLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *UserStore) Authenticate(username, password string) (*User, error) {
	u, ok := s.FindByUsername(username)
	if !ok || !u.Active {
		return nil, fmt.Errorf("username/password salah")
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, fmt.Errorf("username/password salah")
	}
	out := *u
	out.PasswordHash = ""
	return &out, nil
}

func slugUser(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, name)
	name = strings.Trim(name, "-")
	if name == "" {
		name = fmt.Sprintf("user-%d", time.Now().Unix()%100000)
	}
	return name
}

func (s *UserStore) Create(username, password, role, tenantID string) (*User, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	role = strings.TrimSpace(role)
	if username == "" || password == "" {
		return nil, fmt.Errorf("username dan password wajib")
	}
	if role != RoleAdmin && role != RoleTenant {
		role = RoleTenant
	}
	if role == RoleTenant && strings.TrimSpace(tenantID) == "" {
		return nil, fmt.Errorf("tenant_id wajib untuk role tenant")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	want := strings.ToLower(username)
	for _, u := range s.list {
		if strings.ToLower(u.Username) == want {
			return nil, fmt.Errorf("username sudah dipakai")
		}
	}
	id := slugUser(username)
	for _, u := range s.list {
		if u.ID == id {
			id = fmt.Sprintf("%s-%d", id, time.Now().Unix()%1000)
			break
		}
	}
	u := User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		TenantID:     strings.TrimSpace(tenantID),
		Active:       true,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		Billing:      Billing{Status: "active"},
	}
	s.list = append(s.list, u)
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	u.PasswordHash = ""
	return &u, nil
}

func (s *UserStore) Update(id string, mutate func(*User) error) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].ID != id {
			continue
		}
		u := s.list[i]
		if err := mutate(&u); err != nil {
			return nil, err
		}
		s.list[i] = u
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
		u.PasswordHash = ""
		return &u, nil
	}
	return nil, fmt.Errorf("user tidak ditemukan")
}

func (s *UserStore) SetPassword(id, password string) error {
	if strings.TrimSpace(password) == "" {
		return fmt.Errorf("password kosong")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	_, err = s.Update(id, func(u *User) error {
		u.PasswordHash = hash
		return nil
	})
	return err
}
