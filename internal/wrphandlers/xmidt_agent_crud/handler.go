// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package xmidt_agent_crud

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/xmidt-org/wrp-go/v4"
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
	response := msg
	response.Destination = msg.Source
	response.Source = h.source
	response.ContentType = "application/json"
	payload := make(map[string]string)

	err := json.Unmarshal(msg.Payload, &payload)
	if err != nil {
		statusCode := int64(http.StatusInternalServerError)
		response.Status = &statusCode
		response.Payload = []byte(fmt.Sprintf(`{statusCode: %d, message: "%s"}`, statusCode, err.Error()))
		return h.egress.HandleWrp(response)
	}

	statusCode := int64(http.StatusBadRequest)
	payloadResponse := []byte(fmt.Sprintf(`{statusCode: %d, message: "%s"}`, statusCode, ""))

	switch msg.Type {
	case wrp.UpdateMessageType:
		statusCode, err = h.update(msg.Path, payload)
		payloadResponse = []byte(fmt.Sprintf(`{statusCode: %d, message: "%s"}`, statusCode, ""))
		if err != nil {
			payloadResponse = []byte(fmt.Sprintf(`{statusCode: %d, message: "%s"}`, statusCode, err.Error()))
		}

	default:

	}

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
