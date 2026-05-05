package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func gmailConfig() (*oauth2.Config, error) {
	credsJSON := os.Getenv("GOOGLE_CREDENTIALS_JSON")
	if credsJSON == "" {
		return nil, fmt.Errorf("GOOGLE_CREDENTIALS_JSON not set")
	}
	config, err := google.ConfigFromJSON([]byte(credsJSON), gmail.GmailReadonlyScope)
	if err != nil {
		return nil, fmt.Errorf("invalid GOOGLE_CREDENTIALS_JSON: %v", err)
	}
	return config, nil
}

func gmailTokenPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "resumectl", "gmail_token.json")
}

func loadGmailToken() (*oauth2.Token, error) {
	if tokenJSON := os.Getenv("GMAIL_TOKEN_JSON"); tokenJSON != "" {
		var tok oauth2.Token
		if err := json.Unmarshal([]byte(tokenJSON), &tok); err != nil {
			return nil, fmt.Errorf("invalid GMAIL_TOKEN_JSON: %v", err)
		}
		return &tok, nil
	}
	f, err := os.Open(gmailTokenPath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var tok oauth2.Token
	if err := json.NewDecoder(f).Decode(&tok); err != nil {
		return nil, err
	}
	return &tok, nil
}

func saveGmailToken(tok *oauth2.Token) error {
	path := gmailTokenPath()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(tok)
}

func getGmailService() (*gmail.Service, error) {
	config, err := gmailConfig()
	if err != nil {
		return nil, err
	}
	tok, err := loadGmailToken()
	if err != nil {
		return nil, fmt.Errorf("no Gmail token — run: resumectl gmail-auth")
	}
	svc, err := gmail.NewService(context.Background(), option.WithHTTPClient(config.Client(context.Background(), tok)))
	if err != nil {
		return nil, err
	}
	return svc, nil
}

func runGmailAuth(args []string) {
	config, err := gmailConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Visit this URL to authorize Gmail access:\n\n  %s\n\nPaste the auth code: ", authURL)

	var code string
	fmt.Scan(&code)

	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		fmt.Fprintf(os.Stderr, "token exchange failed: %v\n", err)
		os.Exit(1)
	}

	tokenJSON, _ := json.Marshal(tok)
	fmt.Printf("\nAdd to .env or secrets:\n\n  GMAIL_TOKEN_JSON='%s'\n", string(tokenJSON))

	if err := saveGmailToken(tok); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not save token locally: %v\n", err)
	} else {
		fmt.Printf("\nAlso saved to %s\n", gmailTokenPath())
	}
}

type EmailSummary struct {
	From    string
	Subject string
	Snippet string
	Date    string
}

func fetchJobEmails(svc *gmail.Service, query string) ([]EmailSummary, error) {
	result, err := svc.Users.Messages.List("me").Q(query).MaxResults(50).Do()
	if err != nil {
		return nil, err
	}

	var summaries []EmailSummary
	for _, m := range result.Messages {
		msg, err := svc.Users.Messages.Get("me", m.Id).Format("metadata").
			MetadataHeaders("From", "Subject", "Date").Do()
		if err != nil {
			continue
		}
		s := EmailSummary{Snippet: msg.Snippet}
		for _, h := range msg.Payload.Headers {
			switch h.Name {
			case "From":
				s.From = h.Value
			case "Subject":
				s.Subject = h.Value
			case "Date":
				s.Date = h.Value
			}
		}
		summaries = append(summaries, s)
	}
	return summaries, nil
}
