package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/oklog/ulid/v2"
)

type User struct {
	ID    string
	Email string
}

func tokenHash(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func RegisterUser(email string) (token string, err error) {
	token, err = generateToken()
	if err != nil {
		return "", err
	}
	id := ulid.Make().String()
	_, err = db.Exec(
		`INSERT INTO users (id, email, api_token_hash) VALUES ($1, $2, $3)`,
		id, email, tokenHash(token),
	)
	return token, err
}

func UserFromToken(token string) (*User, error) {
	var u User
	err := db.QueryRow(
		`SELECT id, email FROM users WHERE api_token_hash = $1`,
		tokenHash(token),
	).Scan(&u.ID, &u.Email)
	return &u, err
}

func saveTokenLocally(token string) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".resumectl")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	envPath := filepath.Join(dir, ".env")

	existing, _ := os.ReadFile(envPath)
	lines := []string{}
	for _, line := range strings.Split(string(existing), "\n") {
		if !strings.HasPrefix(line, "RESUMECTL_API_TOKEN=") {
			lines = append(lines, line)
		}
	}
	lines = append(lines, "RESUMECTL_API_TOKEN="+token)
	content := strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	return os.WriteFile(envPath, []byte(content+"\n"), 0600)
}

func ensureRegistered() {
	if os.Getenv("RESUMECTL_API_TOKEN") != "" {
		return
	}
	if err := InitDB(); err != nil {
		return
	}

	fmt.Print("Welcome to resumectl. What's your email? ")
	reader := bufio.NewReader(os.Stdin)
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)
	if email == "" {
		fmt.Fprintln(os.Stderr, "error: email required")
		os.Exit(1)
	}

	token, err := RegisterUser(email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error registering: %v\n", err)
		os.Exit(1)
	}
	if err := saveTokenLocally(token); err != nil {
		fmt.Fprintf(os.Stderr, "error saving token: %v\n", err)
		os.Exit(1)
	}

	os.Setenv("RESUMECTL_API_TOKEN", token)
	fmt.Println("✓ Registered. Token saved to ~/.resumectl/.env")
	fmt.Println()
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		http.Error(w, `{"error":"email required"}`, http.StatusBadRequest)
		return
	}

	token, err := RegisterUser(req.Email)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"registration failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
