// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput     = fmt.Errorf("invalid input")
	ErrInvalidFileInput = fmt.Errorf("misconfigured file input")
	ErrUnableToReadFile = fmt.Errorf("unable to read file")
)

const (
	// statusCode is the status code to return when a message is not authorized.
	statusCode = 403

	// wildcard is the wildcard partner id that matches all partner ids.
	wildcard = "*"
)

// Option is a functional option type for WS.
type Option interface {
	apply(*Handler) error
}

type optionFunc func(*Handler) error

func (f optionFunc) apply(c *Handler) error {
	return f(c)
}

// Handler sends a response when a message is required to have a response, but
// was not handled by the next handler in the chain.
type Handler struct {
	next       wrpkit.Handler
	egress     wrpkit.Handler
	source     string
	filePath   string
	parameters *Parameters
}

type Parameter struct {
	name      string
	value     string
	access    string
	paramType int // add json labels here
	delay     int
}

type Parameters struct { // TODO rename
	parameters []Parameter
}

// New creates a new instance of the Handler struct.  The parameter next is the
// handler that will be called and monitored for errors.  The parameter egress is
// the handler that will be called to send the response if/when the next handler
// fails to handle the message.  The parameter source is the source to use in
// the response message.
func New(next, egress wrpkit.Handler, source string, opts ...Option) (*Handler, error) {
	// TODO - load config from file system

	h := Handler{
		next:   next,
		egress: egress,
		source: source,
	}

	for _, opt := range opts {
		if opt != nil {
			if err := opt.apply(&h); err != nil {
				return nil, err
			}
		}
	}

	parameters, err := h.loadFile()
	if err != nil {
		return nil, ErrUnableToReadFile
	}

	h.parameters = parameters

	if h.next == nil || h.egress == nil || h.source == "" {
		return nil, ErrInvalidInput
	}

	return &h, nil
}

// HandleWrp is called to process a message.  If the message is not from an allowed
// partner, a response is sent to the source of the message if applicable.
func (h Handler) HandleWrp(msg wrp.Message) error {
	// TODO - parse message for requested parameters (not really sure what this looks like)
	for _, allowed := range h.partners {
		for _, got := range msg.PartnerIDs {
			got = strings.TrimSpace(got)
			if allowed == got || allowed == wildcard {
				// We found a match, so continue processing the message.
				return h.next.HandleWrp(msg)
			}
		}
	}

	// At this point, the message is not from an allowed partner, so send a
	// response if needed.  Otherwise, return an error.

	if !msg.Type.RequiresTransaction() {
		return ErrUnauthorized
	}

	got := strings.Join(msg.PartnerIDs, "','")
	want := strings.Join(h.partners, "','")

	//no next?  we just send the mocktr181 response here?
	response := msg
	response.Destination = msg.Source
	response.Source = h.source
	response.ContentType = "text/plain"
	response.Payload = []byte(fmt.Sprintf("Partner(s) '%s' not allowed.  Allowed: '%s'", got, want))

	code := int64(statusCode)
	response.Status = &code

	sendErr := h.egress.HandleWrp(response)

	return errors.Join(ErrUnauthorized, sendErr)
}

func (h Handler) loadFile() (*Parameters, error) {
	jsonFile, err := os.Open(h.filePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	var parameters Parameters
	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &parameters)
	if err != nil {
		return nil, err
	}

	return &parameters, nil
}
