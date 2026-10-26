package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	httpadapter "github.com/araihu/balemoh/internal/adapters/http"
	"github.com/araihu/balemoh/internal/adapters/sqlite"
	"github.com/araihu/balemoh/internal/application/notifications"
	"github.com/araihu/balemoh/internal/config"
)

const (
	maxNotificationCredentialFileBytes = 4 << 10
	maxNotificationPublishersFileBytes = 16 << 10
	minNotificationCredentialBytes     = 32
)

// newNotificationsService is the startup seam used by core and future plugin
// callback wiring. It has no dependency on installed plugins.
func newNotificationsService(db *sql.DB) *notifications.Service {
	return notifications.NewService(sqlite.NewNotificationStore(db))
}

func configureNotifications(options config.Options, db *sql.DB) (*notifications.Service, httpadapter.NotificationAuth, error) {
	auth, err := loadNotificationAuth(options)
	if err != nil {
		return nil, httpadapter.NotificationAuth{}, err
	}
	return newNotificationsService(db), auth, nil
}

func loadNotificationAuth(options config.Options) (httpadapter.NotificationAuth, error) {
	operatorToken, err := readNotificationTokenFile(options.NotificationOperatorTokenFile, "operator")
	if err != nil {
		return httpadapter.NotificationAuth{}, err
	}
	publishers := make(map[string]string)
	path := strings.TrimSpace(options.NotificationPublishersFile)
	if path != "" {
		contents, err := readNotificationFile(path, maxNotificationPublishersFileBytes, "publishers")
		if err != nil {
			return httpadapter.NotificationAuth{}, err
		}
		configured, err := decodeNotificationPublishers(contents)
		if err != nil {
			return httpadapter.NotificationAuth{}, err
		}
		for sourceID, tokenPath := range configured {
			sourceID = strings.TrimSpace(sourceID)
			tokenPath = strings.TrimSpace(tokenPath)
			if sourceID == "" || tokenPath == "" {
				return httpadapter.NotificationAuth{}, fmt.Errorf("read notification publishers file: source IDs and token files must not be empty")
			}
			if err := (notifications.Source{Kind: notifications.SourceExternal, ID: sourceID}).Validate(); err != nil {
				return httpadapter.NotificationAuth{}, fmt.Errorf("read notification publishers file: invalid source ID: %w", err)
			}
			token, err := readNotificationTokenFile(tokenPath, "publisher")
			if err != nil {
				return httpadapter.NotificationAuth{}, err
			}
			if operatorToken != "" && token == operatorToken {
				return httpadapter.NotificationAuth{}, fmt.Errorf("notification operator and publisher credentials must be different")
			}
			for existingSource, existingToken := range publishers {
				if token == existingToken {
					return httpadapter.NotificationAuth{}, fmt.Errorf("notification publisher credentials must be unique (source %q and %q)", existingSource, sourceID)
				}
			}
			publishers[sourceID] = token
		}
	}
	return httpadapter.NotificationAuth{OperatorToken: operatorToken, Publishers: publishers}, nil
}

func decodeNotificationPublishers(contents []byte) (map[string]string, error) {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	first, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("read notification publishers file: expected a JSON object mapping source IDs to token files")
	}
	delimiter, ok := first.(json.Delim)
	if !ok || delimiter != '{' {
		return nil, fmt.Errorf("read notification publishers file: expected a JSON object mapping source IDs to token files")
	}
	configured := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("read notification publishers file: invalid JSON object")
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("read notification publishers file: source ID must be a string")
		}
		if _, exists := configured[key]; exists {
			return nil, fmt.Errorf("read notification publishers file: duplicate source ID")
		}
		var tokenPath string
		if err := decoder.Decode(&tokenPath); err != nil {
			return nil, fmt.Errorf("read notification publishers file: token file must be a string")
		}
		configured[key] = tokenPath
	}
	last, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("read notification publishers file: invalid JSON object")
	}
	lastDelimiter, ok := last.(json.Delim)
	if !ok || lastDelimiter != '}' {
		return nil, fmt.Errorf("read notification publishers file: expected one JSON object")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("read notification publishers file: expected one JSON object")
	}
	return configured, nil
}

func readNotificationTokenFile(path, kind string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	contents, err := readNotificationFile(path, maxNotificationCredentialFileBytes, kind+" credential")
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(string(contents))
	if token == "" || strings.ContainsAny(token, " \t\r\n") || strings.ContainsRune(token, '\x00') {
		return "", fmt.Errorf("read notification %s credential: token must be one non-empty value", kind)
	}
	if len(token) < minNotificationCredentialBytes {
		return "", fmt.Errorf("read notification %s credential: token must contain at least %d bytes", kind, minNotificationCredentialBytes)
	}
	return token, nil
}

func readNotificationFile(path string, maxBytes int, kind string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open notification %s file: %w", kind, err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, int64(maxBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read notification %s file: %w", kind, err)
	}
	if len(contents) > maxBytes {
		return nil, fmt.Errorf("notification %s file exceeds %d bytes", kind, maxBytes)
	}
	return contents, nil
}
