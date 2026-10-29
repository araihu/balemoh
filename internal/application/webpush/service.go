// Package webpush owns browser subscriptions and durable delivery policy.
package webpush

import (
	"context"
	"crypto/ecdh"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid push subscription")
	ErrConflict = errors.New("push subscription belongs to another browser")
	ErrCapacity = errors.New("push subscription capacity exceeded")
)

type Subscription struct {
	Endpoint string `json:"endpoint"`
	Keys     Keys   `json:"keys"`
}
type Keys struct {
	P256dh string `json:"p256dh"`
	Auth   string `json:"auth"`
}
type Registration struct {
	Token        string       `json:"token"`
	Subscription Subscription `json:"subscription"`
}
type Settings struct {
	Enabled    bool   `json:"enabled"`
	PublicKey  string `json:"publicKey"`
	Subscribed bool   `json:"subscribed"`
}
type Delivery struct {
	SubscriptionID, NotificationID string
	Subscription                   Subscription
	Title, Body                    string
	Attempts                       int
}
type Store interface {
	Configure(context.Context, string, bool) error
	Save(context.Context, string, Subscription, time.Time) error
	Delete(context.Context, string) error
	Exists(context.Context, string, time.Time) (bool, error)
	Next(context.Context, time.Time) (Delivery, error)
	Complete(context.Context, Delivery, int, time.Time) error
}
type Sender interface {
	Send(context.Context, Delivery) (int, error)
}
type Service struct {
	store     Store
	sender    Sender
	publicKey string
}

func New(store Store, sender Sender, publicKey string) *Service {
	return &Service{store: store, sender: sender, publicKey: publicKey}
}
func (s *Service) Enabled() bool { return s != nil && s.sender != nil && s.publicKey != "" }
func (s *Service) Settings(ctx context.Context, token string) (Settings, error) {
	result := Settings{Enabled: s.Enabled()}
	if s == nil {
		return result, nil
	}
	if result.Enabled {
		result.PublicKey = s.publicKey
	}
	if token != "" {
		id, err := TokenID(token)
		if err != nil {
			return Settings{}, err
		}
		result.Subscribed, err = s.store.Exists(ctx, id, time.Now())
		if err != nil {
			return Settings{}, err
		}
	}
	return result, nil
}
func TokenID(token string) (string, error) {
	if len(token) != 43 {
		return "", ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 {
		return "", ErrInvalid
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "" && (u.Port() == "" || u.Port() == "443") && len(endpoint) <= 2048
}
func (s *Service) Register(ctx context.Context, r Registration) error {
	id, err := TokenID(r.Token)
	if err != nil {
		return err
	}
	if !ValidEndpoint(r.Subscription.Endpoint) {
		return ErrInvalid
	}
	key, err := base64.RawURLEncoding.DecodeString(r.Subscription.Keys.P256dh)
	if err != nil {
		return ErrInvalid
	}
	if _, err = ecdh.P256().NewPublicKey(key); err != nil {
		return ErrInvalid
	}
	auth, err := base64.RawURLEncoding.DecodeString(r.Subscription.Keys.Auth)
	if err != nil || len(auth) != 16 {
		return ErrInvalid
	}
	return s.store.Save(ctx, id, r.Subscription, time.Now().Add(90*24*time.Hour))
}
func (s *Service) Delete(ctx context.Context, token string) error {
	id, err := TokenID(token)
	if err != nil {
		return err
	}
	return s.store.Delete(ctx, id)
}

// Run serializes bounded sends in this process. Durable leases also protect
// against a second API replica and restart. Cancellation joins at the caller.
func (s *Service) Run(ctx context.Context) {
	if !s.Enabled() {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		s.Drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (s *Service) Drain(ctx context.Context) {
	for range 32 {
		if ctx.Err() != nil {
			return
		}
		d, err := s.store.Next(ctx, time.Now())
		if err != nil {
			return
		}
		work, cancel := context.WithTimeout(ctx, 10*time.Second)
		status, sendErr := s.sender.Send(work, d)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if sendErr != nil {
			status = 0
		}
		log.Printf("Web Push delivery result: attempt=%d status=%d", d.Attempts, status)
		// Never log endpoint URLs, key material, payloads or transport errors.
		if err = s.store.Complete(ctx, d, status, time.Now()); err != nil {
			return
		}
	}
}

// ValidSubject accepts only contact URIs suitable for the VAPID subject claim.
func ValidSubject(value string) bool {
	u, err := url.Parse(value)
	if err != nil || strings.TrimSpace(value) != value {
		return false
	}
	return (u.Scheme == "mailto" && u.Opaque != "" && !strings.ContainsAny(u.Opaque, " \r\n")) || (u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Fragment == "")
}
