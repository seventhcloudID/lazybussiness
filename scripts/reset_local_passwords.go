//go:build ignore

package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"threads-dashboard/internal/auth"
)

func randPass() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func main() {
	root := ".data"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	store, err := auth.OpenUsers(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	adminPass := randPass()
	tenantPass := randPass()
	if err := store.SetPassword("admin", adminPass); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
	if err := store.SetPassword("rootedblack", tenantPass); err != nil {
		fmt.Fprintln(os.Stderr, "tenant:", err)
		os.Exit(1)
	}
	out := fmt.Sprintf(`# Local login (reset %s)
# App: http://localhost:8081

## Core (admin)
username: admin
password: %s

## App (tenant)
username: rootedblack@gmail.com
password: %s
`, filepath.Base(root), adminPass, tenantPass)
	path := filepath.Join(root, ".owner-credentials")
	if err := os.WriteFile(path, []byte(out), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Print(out)
	fmt.Println("saved →", path)
}
