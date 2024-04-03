// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
	"go.uber.org/zap"
)

var (
	ErrInvalidInput     = fmt.Errorf("invalid input")
	ErrInvalidFileInput = fmt.Errorf("misconfigured file input")
	ErrUnableToReadFile = fmt.Errorf("unable to read file")
)

// Option is a functional option type for WS.
type Option interface {
	apply(*Handler) error
}

type optionFunc func(*Handler) error

func (f optionFunc) apply(c *Handler) error {
	return f(c)
}

type Handler struct {
	egress     wrpkit.Handler
	source     string
	filePath   string
	parameters []MockParameter
	logger     *zap.Logger
	enabled    bool
}

type MockParameter struct {
	Name       string
	Value      string
	Access     string
	DataType   int // add json labels here
	Attributes map[string]interface{}
	Delay      int
}

type MockParameters struct {
	Parameters []MockParameter
}

type Tr181Payload struct {
	Command    string      `json:"command"`
	Names      []string    `json:"names"`
	Parameters []Parameter `json:"parameters"`
}

type Parameters struct {
	Parameters []Parameter
}

type Parameter struct {
	Name       string                 `json:"name"`
	Value      string                 `json:"value"`
	DataType   int                    `json:"dataType"`
	Attributes map[string]interface{} `json:"attributes"`
}

// New creates a new instance of the Handler struct.  The parameter egress is
// the handler that will be called to send the response.  The parameter source is the source to use in
// the response message.
func New(egress wrpkit.Handler, source string, logger *zap.Logger, opts ...Option) (*Handler, error) {
	// TODO - load config from file system

	h := Handler{
		egress: egress,
		source: source,
		logger: logger,
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
		h.logger.Error("unable to load mock data", zap.Error(err))
		return nil, ErrUnableToReadFile
	}

	h.parameters = parameters

	if h.egress == nil || h.source == "" {
		return nil, ErrInvalidInput
	}

	return &h, nil
}

func (h Handler) Enabled() bool {
	return h.enabled
}

// HandleWrp is called to process a tr181 command
func (h Handler) HandleWrp(msg wrp.Message) error {
	payload := new(Tr181Payload)
	err := json.Unmarshal(msg.Payload, &payload)
	if err != nil {
		h.logger.Error("unable to unmarshal msg payload", zap.Error(err))
		return err
	}

	var payloadResponse []byte
	var statusCode int64

	command := payload.Command

	switch command {
	case "GET":
		statusCode, payloadResponse = h.get(payload.Names)

	case "SET":
		statusCode = h.set(payload.Parameters)

	default:
		// currently only get and set are implemented for existing mocktr181
		statusCode = http.StatusOK
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
	result := []Parameter{}

	for _, name := range names {
		for _, mockParameter := range h.parameters {
			if strings.HasPrefix(mockParameter.Name, name) {
				result = append(result, Parameter{
					Name:       mockParameter.Name,
					Value:      mockParameter.Value,
					DataType:   mockParameter.DataType,
					Attributes: mockParameter.Attributes,
				})
			}
		}
	}

	payload, err := json.Marshal(result)
	if err != nil {
		h.logger.Error("unable to marshal get result", zap.Error(err))
		return http.StatusInternalServerError, payload
	}

	return http.StatusOK, payload
}

func (h Handler) set(parameters []Parameter) int64 {
	for _, parameter := range parameters {
		for _, mockParameter := range h.parameters {
			if strings.HasPrefix(mockParameter.Name, parameter.Name) {
				if mockParameter.Access == "rw" {
					mockParameter.Value = parameter.Value
					mockParameter.DataType = parameter.DataType
					mockParameter.Attributes = parameter.Attributes
				}
			}
		}
	}

	return http.StatusAccepted
}

func (h Handler) loadFile() ([]MockParameter, error) {
	jsonFile, err := os.Open(h.filePath)
	if err != nil {
		h.logger.Error("unable to open mock parameter file", zap.Error(err))
		return nil, err
	}
	defer jsonFile.Close()

	var parameters []MockParameter
	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &parameters)
	if err != nil {
		h.logger.Error("unable to unmarshal mock parameter file", zap.Error(err))
		return nil, err
	}

	return parameters, nil
}
