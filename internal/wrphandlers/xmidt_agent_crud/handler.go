// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package xmidt_agent_crud

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/loglevel"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

const DefaultLogLevelChangeMinutes = 30

const (
	Create   = 5
	Retrieve = 6
	Update   = 7
	Delete   = 8
)

type Handler struct {
	egress   wrpkit.Handler
	source   string
	logLevel loglevel.LogLevel
}

// New creates a new instance of the Handler struct.  The parameter egress is
// the handler that will be called to send the response.  The parameter source is the source to use in
// the response message. This handler handles crud messages specifically for xmdit-agent, only.
func New(egress wrpkit.Handler, source string, logLevel loglevel.LogLevel) (*Handler, error) {

	h := Handler{
		egress:   egress,
		source:   source,
		logLevel: logLevel,
	}

	return &h, nil
}

func (h *Handler) HandleWrp(msg wrp.Message) error {
	payload := make(map[string]interface{})

	err := json.Unmarshal(msg.Payload, &payload)
	if err != nil { // do we still want to return a response here?
		return err // this would be sent to the eventor error handler
	}

	var payloadResponse []byte
	statusCode := int64(http.StatusOK)

	switch msg.Type {
	case Update:
		statusCode, _ = h.update(msg.Path, payload)
		// the above error needs to be logged

	default:

	}

	response := msg
	response.Destination = msg.Source
	response.Source = h.source
	response.ContentType = "text/plain"
	response.Payload = payloadResponse

	response.Status = &statusCode

	err = h.egress.HandleWrp(response)

	return err
}

// why is status code int64?
func (h *Handler) update(path string, payload map[string]interface{}) (int64, error) {
	switch path {
	case "loglevel":
		err := h.changeLogLevel(payload)
		if err != nil {
			return int64(http.StatusBadRequest), err
		}

	default:
		return int64(http.StatusOK), nil
	}

	return int64(http.StatusOK), nil
}

func (h *Handler) changeLogLevel(payload map[string]interface{}) error {
	minutes := float64(DefaultLogLevelChangeMinutes)
	inputMinutes := payload["duration"]
	if inputMinutes != nil {
		minutes = inputMinutes.(float64)
	}

	duration := time.Duration(minutes) * time.Minute
	return h.logLevel.SetLevel(payload["loglevel"].(string), duration)
}
