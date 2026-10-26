package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/araihu/balemoh/internal/config"
)

func TestLoadNotificationAuthStrictlySeparatesCredentialFiles(t *testing.T) {
	directory := t.TempDir()
	operatorPath := filepath.Join(directory, "operator.token")
	publisherPath := filepath.Join(directory, "publisher.token")
	publishersPath := filepath.Join(directory, "publishers.json")
	operatorToken := "operator-token-value-123456789012345"
	publisherToken := "publisher-token-value-123456789012345"
	if err := os.WriteFile(operatorPath, []byte(operatorToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publisherPath, []byte(publisherToken+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publishersPath, []byte(`{"monitor":"`+publisherPath+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := loadNotificationAuth(config.Options{NotificationOperatorTokenFile: operatorPath, NotificationPublishersFile: publishersPath})
	if err != nil {
		t.Fatal(err)
	}
	if auth.OperatorToken != operatorToken || auth.Publishers["monitor"] != publisherToken {
		t.Fatalf("notification auth = %#v", auth)
	}
}

func TestLoadNotificationAuthEmptyPublishersDisablesExternalPublishing(t *testing.T) {
	auth, err := loadNotificationAuth(config.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if auth.OperatorToken != "" || len(auth.Publishers) != 0 {
		t.Fatalf("empty notification auth = %#v", auth)
	}
}

func TestLoadNotificationAuthRejectsMalformedAndCrossClassCredentials(t *testing.T) {
	directory := t.TempDir()
	operatorPath := filepath.Join(directory, "operator.token")
	publisherPath := filepath.Join(directory, "publisher.token")
	sameToken := "same-token-value-123456789012345"
	if err := os.WriteFile(operatorPath, []byte(sameToken), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publisherPath, []byte(sameToken), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, contents := range []string{`[]`, `{"monitor":""}`, `{"monitor":"` + filepath.Join(directory, "missing") + `"}`, `{"monitor":"` + publisherPath + `"}`, `{"monitor":"` + publisherPath + `","monitor":"` + publisherPath + `"}`} {
		publishersPath := filepath.Join(directory, "publishers-"+string(rune(len(contents)))+".json")
		if err := os.WriteFile(publishersPath, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := loadNotificationAuth(config.Options{NotificationOperatorTokenFile: operatorPath, NotificationPublishersFile: publishersPath})
		if err == nil {
			t.Fatalf("loadNotificationAuth(%q) succeeded", contents)
		}
	}
}

func TestLoadNotificationAuthRejectsShortCredential(t *testing.T) {
	directory := t.TempDir()
	operatorPath := filepath.Join(directory, "operator.token")
	if err := os.WriteFile(operatorPath, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNotificationAuth(config.Options{NotificationOperatorTokenFile: operatorPath}); err == nil {
		t.Fatal("loadNotificationAuth accepted a short operator credential")
	}

	publisherPath := filepath.Join(directory, "publisher.token")
	if err := os.WriteFile(publisherPath, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	publishersPath := filepath.Join(directory, "publishers.json")
	if err := os.WriteFile(publishersPath, []byte(`{"monitor":"`+publisherPath+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadNotificationAuth(config.Options{NotificationPublishersFile: publishersPath}); err == nil {
		t.Fatal("loadNotificationAuth accepted a short publisher credential")
	}
}
