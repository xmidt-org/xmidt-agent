// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
	parameters *MockParameters
}

type MockParameter struct {
	Name     string
	Value    string
	Access   string
	DataType int // add json labels here
	Delay    int
}

type MockParameters struct {
	Parameters []MockParameter
}

type Payload struct {
	Command    string      `json:"command"`
	Names      []string    `json:"names"`
	Parameters []Parameter `json:"parameters"`
}

type Parameter struct {
	Name       string                 `json:"name"`
	Value      string                 `json:"value"`
	DataType   int                    `json:"dataType"`
	Attributes map[string]interface{} `json:"attributes"`
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

// HandleWrp is called to process a tr181 command
func (h Handler) HandleWrp(msg wrp.Message) error {
	payload := make(map[string]interface{})
	err := json.Unmarshal(msg.Payload, &payload)
	if err != nil {
		// TODO - need logger
		return err
	}

	command := payload["command"].(string)
	var payloadResponse []byte
	var statusCode int64

	switch command {
	case "GET":
		statusCode, payloadResponse = h.get(payload["names"].([]string))

	case "SET":
		statusCode = h.set(payload["parameters"].([]Parameter))

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

func (h Handler) get(names []string) (int64, []byte) {
	var payload []byte

	return http.StatusAccepted, payload
}

func (h Handler) set(parameters []Parameter) int64 {

	return http.StatusOK
}

func (h Handler) loadFile() (*MockParameters, error) {
	jsonFile, err := os.Open(h.filePath)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	var parameters MockParameters
	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &parameters)
	if err != nil {
		return nil, err
	}

	return &parameters, nil
}
