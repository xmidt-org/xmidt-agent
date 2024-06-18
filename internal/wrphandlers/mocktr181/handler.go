// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

var (
	ErrInvalidInput           = fmt.Errorf("invalid input")
	ErrInvalidFileInput       = fmt.Errorf("misconfigured file input")
	ErrUnableToReadFile       = fmt.Errorf("unable to read file")
	ErrInvalidPayload         = fmt.Errorf("invalid request payload")
	ErrInvalidResponsePayload = fmt.Errorf("invalid response payload")
)

// Option is a functional option type for mocktr181 Handler.
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
	Message    string                 `json:"message"`
}

// New creates a new instance of the Handler struct.  The parameter egress is
// the handler that will be called to send the response.  The parameter source is the source to use in
// the response message.
func New(egress wrpkit.Handler, source string, opts ...Option) (*Handler, error) {
	// TODO - load config from file system

	h := Handler{
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
		return nil, errors.Join(ErrUnableToReadFile, err)
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
		return err
	}

	var payloadResponse []byte
	var statusCode int64

	command := payload.Command

	switch command {
	case "GET":
		statusCode, payloadResponse, err = h.get(payload.Names)
		if err != nil {
			return err
		}

	case "SET":
		statusCode, payloadResponse, err = h.set(payload.Parameters)
		if err != nil {
			return err
		}

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

func (h Handler) get(names []string) (int64, []byte, error) {
	result := Tr181Payload{}
	statusCode := int64(http.StatusOK)

	for _, name := range names {
		for _, mockParameter := range h.parameters {
			if !strings.HasPrefix(mockParameter.Name, name) {
				continue
			}

			switch mockParameter.Access {
			case "r", "rw", "wr":
				result.Parameters = append(result.Parameters, Parameter{
					Name:       mockParameter.Name,
					Value:      mockParameter.Value,
					DataType:   mockParameter.DataType,
					Attributes: mockParameter.Attributes,
				})
			default:
				result.Parameters = append(result.Parameters, Parameter{
					Message: fmt.Sprintf("Invalid parameter name: %s", mockParameter.Name),
				})
				statusCode = 520
			}
		}
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	if len(result.Parameters) == 0 {
		statusCode = int64(520)
	}

	return statusCode, payload, nil
}

func (h Handler) set(parameters []Parameter) (int64, []byte, error) {
	statusCode := http.StatusAccepted
	result := Tr181Payload{}
	for _, parameter := range parameters {
		for i := range h.parameters {
			mockParameter := &h.parameters[i]
			if mockParameter.Name != parameter.Name {
				continue
			}

			switch mockParameter.Access {
			case "w", "wr", "rw":
				mockParameter.Value = parameter.Value
				mockParameter.DataType = parameter.DataType
				mockParameter.Attributes = parameter.Attributes

				result.Parameters = append(result.Parameters, Parameter{
					Name:       mockParameter.Name,
					Value:      mockParameter.Value,
					DataType:   mockParameter.DataType,
					Attributes: mockParameter.Attributes,
				})
			default:
				result.Parameters = append(result.Parameters, Parameter{
					Name:    mockParameter.Name,
					Message: "Parameter is not writable",
				})
				statusCode = 520
			}
		}
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(statusCode), payload, nil
}

func (h Handler) loadFile() ([]MockParameter, error) {
	jsonFile, err := os.Open(h.filePath)
	if err != nil {
		return nil, errors.Join(ErrUnableToReadFile, err)
	}
	defer jsonFile.Close()

	var parameters []MockParameter
	byteValue, _ := io.ReadAll(jsonFile)
	err = json.Unmarshal(byteValue, &parameters)
	if err != nil {
		return nil, errors.Join(ErrInvalidFileInput, err)
	}

	return parameters, nil
}
