package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/araihu/balemoh/internal/api/generated"
	"github.com/araihu/balemoh/internal/application/notifications"
	"github.com/google/uuid"
)

const (
	maxNotificationRequestBytes = 16 << 10
	notificationPublishDeadline = 5 * time.Second
)

func (h Handler) GetNotifications(w http.ResponseWriter, r *http.Request, params generated.GetNotificationsParams) {
	setNotificationRequestID(w)
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	options, err := notificationListOptions(params)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{Code: "invalid_options", Message: "invalid notification page options"})
		return
	}
	if h.notifications == nil {
		writeNotificationUnavailable(w)
		return
	}
	page, err := h.notifications.List(r.Context(), options)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	response := generated.NotificationPage{Notifications: make([]generated.Notification, 0, len(page.Notifications)), UnreadCount: int32(page.UnreadCount)}
	for _, notification := range page.Notifications {
		response.Notifications = append(response.Notifications, notificationResponse(notification))
	}
	if page.NextBefore > 0 {
		cursor := encodeNotificationCursor(page.NextBefore)
		response.NextCursor = &cursor
	}
	writeJSON(w, http.StatusOK, response)
}

func (h Handler) GetNotification(w http.ResponseWriter, r *http.Request, id string) {
	setNotificationRequestID(w)
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	if h.notifications == nil {
		writeNotificationUnavailable(w)
		return
	}
	notification, err := h.notifications.Get(r.Context(), id)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, notificationResponse(notification))
}

func (h Handler) MarkNotificationRead(w http.ResponseWriter, r *http.Request, id string) {
	setNotificationRequestID(w)
	if !h.notificationOperatorAuth(w, r) {
		return
	}
	if h.notifications == nil {
		writeNotificationUnavailable(w)
		return
	}
	if err := h.notifications.MarkRead(r.Context(), id); err != nil {
		writeNotificationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) PublishNotification(w http.ResponseWriter, r *http.Request) {
	setNotificationRequestID(w)
	sourceID, ok := h.notificationPublisherAuth(w, r)
	if !ok {
		return
	}
	if h.notifications == nil {
		writeNotificationUnavailable(w)
		return
	}
	slot := h.notificationSlots[sourceID]
	if slot == nil {
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, generated.ErrorResponse{Code: "publisher_busy", Message: "notification publisher is busy"})
		return
	}
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	default:
		w.Header().Set("Retry-After", "1")
		writeJSON(w, http.StatusTooManyRequests, generated.ErrorResponse{Code: "publisher_busy", Message: "notification publisher is busy"})
		return
	}

	request, ok := decodeNotificationPublishRequest(w, r)
	if !ok {
		return
	}
	message, err := notificationMessage(request)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{Code: "invalid_notification", Message: "invalid notification envelope"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), notificationPublishDeadline)
	defer cancel()
	receipt, err := h.notifications.Publish(ctx, notifications.Source{Kind: notifications.SourceExternal, ID: sourceID}, message)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	status := http.StatusOK
	if receipt.Accepted {
		status = http.StatusCreated
	}
	writeJSON(w, status, generated.NotificationReceipt{Id: parseUUID(receipt.ID), CreatedAt: receipt.CreatedAt, Accepted: receipt.Accepted})
}

func (h Handler) notificationOperatorAuth(w http.ResponseWriter, r *http.Request) bool {
	header := r.Header.Get("Authorization")
	if h.notificationAuth.OperatorToken == "" || !validBearerToken(header, h.notificationAuth.OperatorToken) || h.notificationTokenIsPublisher(header) {
		writeJSON(w, http.StatusUnauthorized, generated.ErrorResponse{Code: "unauthorized", Message: "invalid notification operator credentials"})
		return false
	}
	return true
}

func (h Handler) notificationPublisherAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	if len(h.notificationAuth.Publishers) == 0 {
		writeJSON(w, http.StatusNotFound, generated.ErrorResponse{Code: "publisher_disabled", Message: "external notification publishing is disabled"})
		return "", false
	}
	provided, ok := bearerValue(r.Header.Get("Authorization"))
	if !ok || h.notificationAuth.OperatorToken != "" && tokenEqual(provided, h.notificationAuth.OperatorToken) {
		writeJSON(w, http.StatusUnauthorized, generated.ErrorResponse{Code: "unauthorized", Message: "invalid notification publisher credentials"})
		return "", false
	}
	for sourceID, expected := range h.notificationAuth.Publishers {
		if tokenEqual(provided, expected) {
			return sourceID, true
		}
	}
	writeJSON(w, http.StatusUnauthorized, generated.ErrorResponse{Code: "unauthorized", Message: "invalid notification publisher credentials"})
	return "", false
}

func (h Handler) notificationTokenIsPublisher(header string) bool {
	provided, ok := bearerValue(header)
	if !ok {
		return false
	}
	for _, expected := range h.notificationAuth.Publishers {
		if tokenEqual(provided, expected) {
			return true
		}
	}
	return false
}

func decodeNotificationPublishRequest(w http.ResponseWriter, r *http.Request) (generated.NotificationPublishRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxNotificationRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var request generated.NotificationPublishRequest
	if err := decoder.Decode(&request); err != nil {
		if isRequestTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, generated.ErrorResponse{Code: "payload_too_large", Message: "notification exceeds the maximum size"})
		} else {
			writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{Code: "invalid_notification", Message: "invalid notification envelope"})
		}
		return generated.NotificationPublishRequest{}, false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		if isRequestTooLarge(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, generated.ErrorResponse{Code: "payload_too_large", Message: "notification exceeds the maximum size"})
		} else {
			writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{Code: "invalid_notification", Message: "invalid notification envelope"})
		}
		return generated.NotificationPublishRequest{}, false
	}
	return request, true
}

func notificationMessage(request generated.NotificationPublishRequest) (notifications.Message, error) {
	message := notifications.Message{
		EventID:    request.EventId,
		Severity:   notifications.Severity(request.Severity),
		Title:      request.Title,
		OccurredAt: request.OccurredAt,
	}
	if request.Body != nil {
		message.Body = *request.Body
	}
	if request.Destination != nil {
		message.Destination = notificationDestination(*request.Destination)
	}
	return message, nil
}

func notificationListOptions(params generated.GetNotificationsParams) (notifications.ListOptions, error) {
	options := notifications.ListOptions{Limit: notifications.MaxPageSize}
	if params.Limit != nil {
		if *params.Limit < 1 || *params.Limit > notifications.MaxPageSize {
			return notifications.ListOptions{}, notifications.ErrInvalid
		}
		options.Limit = int(*params.Limit)
	}
	if params.Unread != nil {
		options.UnreadOnly = *params.Unread
	}
	if params.Before != nil && strings.TrimSpace(*params.Before) != "" {
		before, err := decodeNotificationCursor(*params.Before)
		if err != nil {
			return notifications.ListOptions{}, err
		}
		options.Before = before
	} else if params.Before != nil {
		return notifications.ListOptions{}, notifications.ErrInvalid
	}
	return options, nil
}

func encodeNotificationCursor(sequence uint64) string {
	return base64.RawURLEncoding.EncodeToString([]byte("notification:" + strconv.FormatUint(sequence, 10)))
}

func decodeNotificationCursor(cursor string) (uint64, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || !strings.HasPrefix(string(decoded), "notification:") {
		return 0, notifications.ErrInvalid
	}
	sequence, err := strconv.ParseUint(strings.TrimPrefix(string(decoded), "notification:"), 10, 63)
	if err != nil || sequence == 0 {
		return 0, notifications.ErrInvalid
	}
	return sequence, nil
}

func notificationDestination(destination generated.NotificationDestination) notifications.Destination {
	result := notifications.Destination{Kind: string(destination.Kind)}
	if destination.Route != nil {
		result.Route = *destination.Route
	}
	if destination.PluginId != nil {
		result.PluginID = *destination.PluginId
	}
	if destination.ConnectionId != nil {
		result.ConnectionID = *destination.ConnectionId
	}
	if destination.Page != nil {
		result.Page = *destination.Page
	}
	if destination.Params != nil {
		result.Params = *destination.Params
	}
	if destination.Query != nil {
		result.Query = *destination.Query
	}
	if destination.Url != nil {
		result.URL = *destination.Url
	}
	return result
}

func notificationResponse(notification notifications.Notification) generated.Notification {
	response := generated.Notification{
		Id:         parseUUID(notification.ID),
		Source:     generated.NotificationSource{Kind: generated.NotificationSourceKind(notification.Source.Kind), Id: notification.Source.ID},
		EventId:    notification.EventID,
		Severity:   generated.NotificationSeverity(notification.Severity),
		Title:      notification.Title,
		Body:       notification.Body,
		OccurredAt: notification.OccurredAt,
		ReceivedAt: notification.ReceivedAt,
		ReadAt:     notification.ReadAt,
	}
	if notification.Source.ConnectionID != "" {
		connectionID := notification.Source.ConnectionID
		response.Source.ConnectionId = &connectionID
	}
	if !notification.Destination.IsZero() {
		destination := notificationDestinationResponse(notification.Destination)
		response.Destination = &destination
	}
	return response
}

func parseUUID(value string) uuid.UUID {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

func notificationDestinationResponse(destination notifications.Destination) generated.NotificationDestination {
	response := generated.NotificationDestination{Kind: generated.NotificationDestinationKind(destination.Kind)}
	if destination.Route != "" {
		value := destination.Route
		response.Route = &value
	}
	if destination.PluginID != "" {
		value := destination.PluginID
		response.PluginId = &value
	}
	if destination.ConnectionID != "" {
		value := destination.ConnectionID
		response.ConnectionId = &value
	}
	if destination.Page != "" {
		value := destination.Page
		response.Page = &value
	}
	if len(destination.Params) > 0 {
		values := mapsCopy(destination.Params)
		response.Params = &values
	}
	if len(destination.Query) > 0 {
		values := queryCopy(destination.Query)
		response.Query = &values
	}
	if destination.URL != "" {
		value := destination.URL
		response.Url = &value
	}
	return response
}

func mapsCopy(values map[string]string) map[string]string {
	copyValues := make(map[string]string, len(values))
	for key, value := range values {
		copyValues[key] = value
	}
	return copyValues
}

func queryCopy(values map[string][]string) map[string][]string {
	copyValues := make(map[string][]string, len(values))
	for key, entries := range values {
		copyValues[key] = append([]string(nil), entries...)
	}
	return copyValues
}

func bearerValue(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func setNotificationRequestID(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	id := w.Header().Get("X-Request-ID")
	if _, err := uuid.Parse(id); err != nil {
		w.Header().Set("X-Request-ID", uuid.New().String())
	}
}

func tokenEqual(provided, expected string) bool {
	return validBearerToken("Bearer "+provided, expected)
}

func writeNotificationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notifications.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, generated.ErrorResponse{Code: "invalid_notification", Message: "invalid notification"})
	case errors.Is(err, notifications.ErrConflict):
		writeJSON(w, http.StatusConflict, generated.ErrorResponse{Code: "identity_conflict", Message: "notification event identity conflicts with existing content"})
	case errors.Is(err, notifications.ErrCapacity):
		writeJSON(w, http.StatusTooManyRequests, generated.ErrorResponse{Code: "capacity", Message: "notification inbox capacity reached"})
	case errors.Is(err, notifications.ErrNotFound):
		writeJSON(w, http.StatusNotFound, generated.ErrorResponse{Code: "not_found", Message: "notification not found"})
	default:
		if errors.Is(err, context.DeadlineExceeded) {
			writeJSON(w, http.StatusServiceUnavailable, generated.ErrorResponse{Code: "deadline_exceeded", Message: "notification service unavailable"})
			return
		}
		writeNotificationUnavailable(w)
	}
}

func writeNotificationUnavailable(w http.ResponseWriter) {
	writeJSON(w, http.StatusServiceUnavailable, generated.ErrorResponse{Code: "unavailable", Message: "notification service unavailable"})
}

var _ generated.ServerInterface = Handler{}
