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
	if err := os.WriteFile(operatorPath, []byte("operator-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publisherPath, []byte("publisher-value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publishersPath, []byte(`{"monitor":"`+publisherPath+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	auth, err := loadNotificationAuth(config.Options{NotificationOperatorTokenFile: operatorPath, NotificationPublishersFile: publishersPath})
	if err != nil {
		t.Fatal(err)
	}
	if auth.OperatorToken != "operator-value" || auth.Publishers["monitor"] != "publisher-value" {
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
	if err := os.WriteFile(operatorPath, []byte("same-value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(publisherPath, []byte("same-value"), 0o600); err != nil {
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
