package bff

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	api "github.com/araihu/balemoh/client"
)

func (s *server) newService(w http.ResponseWriter, r *http.Request) {
	var key [16]byte
	_, _ = rand.Read(key[:])
	service := mapService(api.ServiceCandidate{Source: api.SourceRef{Kind: "manual", Id: "local"}})
	service.Creating, service.CreateKey = true, hex.EncodeToString(key[:])
	s.renderEditor(w, r, service, http.StatusOK)
}

func (s *server) createService(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	service := mapService(api.ServiceCandidate{Source: api.SourceRef{Kind: "manual", Id: "local"}})
	service.Creating = true
	valid := readServiceDraft(w, r, &service)
	service.CreateKey = r.PostForm.Get("requestId")
	key, err := hex.DecodeString(service.CreateKey)
	if err != nil || len(key) != 16 {
		// An invalid form cannot pick a discovered identity. Preserve the draft and
		// give the operator a fresh key for a valid retry.
		var replacement [16]byte
		_, _ = rand.Read(replacement[:])
		service.CreateKey = hex.EncodeToString(replacement[:])
		service.EditError = "The form expired. Review your draft and submit again."
		valid = false
	}
	if !valid {
		s.renderEditor(w, r, service, http.StatusBadRequest)
		return
	}
	created, err := s.catalog.CreateManual(ctx, api.ManualServiceCreate{RequestId: service.CreateKey, DisplayName: service.DisplayName, Description: service.Description, Address: service.Address, Icon: &service.IconRef})
	if err != nil {
		status := http.StatusServiceUnavailable
		service.EditError = "Service could not be saved. Your draft is still here; try again."
		var upstream *upstreamError
		if errors.As(err, &upstream) && upstream.status == http.StatusBadRequest {
			status = http.StatusBadRequest
			service.EditError = "Check the name, address and selected icon, then try again."
		}
		s.renderEditor(w, r, service, status)
		return
	}
	redirect(w, r, "/staging?notice=created#select-"+created.Id)
}
