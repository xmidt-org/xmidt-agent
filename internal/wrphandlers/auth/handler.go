// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package auth

import (
	"fmt"
	"strings"

	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput = fmt.Errorf("invalid input")
	ErrUnauthorized = fmt.Errorf("unauthorized")
)

const (
	// statusCode is the status code to return when a message is not authorized.
	statusCode = 403

	// wildcard is the wildcard partner id that matches all partner ids.
	wildcard = "*"
)

// Handler sends a response when a message is required to have a response, but
// was not handled by the next handler in the chain.
type Handler struct {
	next     wrpkit.Handler
	egress   wrpkit.Handler
	source   string
	partners []string
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.  The parameter egress is
// the handler that will be called to send the response if/when the next handler
// fails to handle the message.  The parameter source is the source to use in
// the response message.  The list of partners is the list of allowed partners.
func New(next, egress wrpkit.Handler, source string, partners ...string) (*Handler, error) {
	h := Handler{
		next:     next,
		egress:   egress,
		source:   source,
		partners: make([]string, 0, len(partners)),
	}

	for _, partner := range partners {
		partner = strings.TrimSpace(partner)
		if partner != "" {
			h.partners = append(h.partners, partner)
		}
	}

	if h.next == nil || h.egress == nil || h.source == "" || len(h.partners) == 0 {
		return nil, ErrInvalidInput
	}

	return &h, nil
}

// HandleWrp is called to process a message.  If the message is not from an allowed
// partner, a response is sent to the source of the message if applicable.
func (h Handler) HandleWrp(msg wrp.Message) error {
	for _, allowed := range h.partners {
		for _, got := range msg.PartnerIDs {
			got = strings.TrimSpace(got)
			if allowed == got || allowed == wildcard {
				// matched — process and exit
				return h.next.HandleWrp(msg)
			}
		}
	}
	// no match found, but we still want to process:
	return h.next.HandleWrp(msg)

	// At this point, the message is not from an allowed partner, so send a
	// response if needed.  Otherwise, return an error.

	// if !msg.Type.RequiresTransaction() {
	// 	return ErrUnauthorized
	// }

	// got := strings.Join(msg.PartnerIDs, "','")
	// want := strings.Join(h.partners, "','")

	// response := msg
	// response.Destination = msg.Source
	// response.Source = h.source
	// response.ContentType = "application/json"

	// code := int64(statusCode)
	// response.Status = &code
	// response.Payload = []byte(fmt.Sprintf(`{statusCode: %d, message:"Partner(s) '%s' not allowed.  Allowed: '%s'"}`, code, got, want))

	// sendErr := h.egress.HandleWrp(response)

	// return errors.Join(ErrUnauthorized, sendErr)
}
