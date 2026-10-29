// Package webpush implements encrypted RFC 8291 delivery with VAPID.
package webpush

import (
	"context"
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	protocol "github.com/SherClockHolmes/webpush-go"
	push "github.com/araihu/balemoh/internal/application/webpush"
)

type Sender struct {
	publicKey, privateKey, subject string
	client                         protocol.HTTPClient
}

func New(privateKey, subject string) (*Sender, error) {
	raw, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil || len(raw) != 32 || !push.ValidSubject(subject) {
		return nil, errors.New("invalid Web Push VAPID configuration")
	}
	key, err := ecdh.P256().NewPrivateKey(raw)
	if err != nil {
		return nil, errors.New("invalid Web Push VAPID private key")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil // Endpoints must never route through a proxy that bypasses address validation.
	transport.DialContext = publicDial
	return &Sender{privateKey: privateKey, publicKey: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), subject: subject, client: &http.Client{Transport: transport, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (s *Sender) PublicKey() string { return s.publicKey }

var excludedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

func publicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	if ip.Is6() && !netip.MustParsePrefix("2000::/3").Contains(ip) {
		return false
	}
	for _, prefix := range excludedNetworks {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, errors.New("invalid push endpoint address")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("push endpoint resolution failed")
	}
	if len(ips) == 0 {
		return nil, errors.New("push endpoint has no addresses")
	}
	for _, ip := range ips {
		if !publicAddress(ip) {
			return nil, errors.New("push endpoint must resolve to public addresses")
		}
	}
	var dialer net.Dialer
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, errors.New("push endpoint connection failed")
}
func (s *Sender) Send(ctx context.Context, d push.Delivery) (int, error) {
	if !push.ValidEndpoint(d.Subscription.Endpoint) {
		return 400, nil
	}
	// Keep encrypted payload below the mandatory 4096-byte push-service limit.
	body := []rune(d.Body)
	if len(body) > 500 {
		body = append(body[:500], '…')
	}
	title := []rune(d.Title)
	if len(title) > 120 {
		title = append(title[:120], '…')
	}
	payload, err := json.Marshal(struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Body  string `json:"body"`
	}{d.NotificationID, string(title), string(body)})
	if err != nil {
		return 0, err
	}
	response, err := protocol.SendNotificationWithContext(ctx, payload, &protocol.Subscription{Endpoint: d.Subscription.Endpoint, Keys: protocol.Keys{P256dh: d.Subscription.Keys.P256dh, Auth: d.Subscription.Keys.Auth}}, &protocol.Options{HTTPClient: s.client, Subscriber: s.subject, VAPIDPublicKey: s.publicKey, VAPIDPrivateKey: s.privateKey, TTL: 86400, RecordSize: 4096, Urgency: protocol.UrgencyNormal})
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	return response.StatusCode, nil
}
