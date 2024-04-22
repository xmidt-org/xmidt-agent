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

const DefaultLogLevelChangeDuration = 30 * time.Minute

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
	payload := make(map[string]string)

	err := json.Unmarshal(msg.Payload, &payload)
	if err != nil { // do we still want to return a response here?
		return err // this would be sent to the eventor error handler
	}

	var payloadResponse []byte
	statusCode := int64(http.StatusBadRequest)

	switch msg.Type {
	case wrp.UpdateMessageType:
		statusCode, err = h.update(msg.Path, payload)
		if err != nil {
			payloadResponse = []byte(err.Error())
		}

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

func (h *Handler) update(path string, payload map[string]string) (int64, error) {
	badRequestStatus := int64(http.StatusBadRequest)
	okStatus := int64(http.StatusOK)

	switch path {
	case "loglevel":
		err := h.changeLogLevel(payload)
		if err != nil {
			return badRequestStatus, err
		}
		return okStatus, nil

	default:
		return badRequestStatus, nil
	}

}

func (h *Handler) changeLogLevel(payload map[string]string) error {
	duration, err := time.ParseDuration(payload["duration"])
	if err != nil {
		duration = DefaultLogLevelChangeDuration
	}

	return h.logLevel.SetLevel(payload["loglevel"], duration)
}
