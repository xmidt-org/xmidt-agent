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

	"github.com/xmidt-org/wrp-go/v5"
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
	StatusCode int         `json:"statusCode"`
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
	Count      int                    `json:"parameterCount"`
}

// New creates a new instance of the Handler struct.  The parameter egress is
// the handler that will be called to send the response.  The parameter source is the source to use in
// the response message.
func New(egress wrpkit.Handler, source string, opts ...Option) (*Handler, error) {
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
	_, payloadResponse, err := h.proccessCommand(msg.Payload)
	if err != nil {
		return errors.Join(err, wrpkit.ErrNotHandled)
	}

	response := msg
	response.Destination = msg.Source
	response.Source = h.source
	response.ContentType = "application/json"
	response.Payload = payloadResponse
	if err = h.egress.HandleWrp(response); err != nil {
		return errors.Join(err, wrpkit.ErrNotHandled)
	}

	return nil
}

func (h Handler) proccessCommand(wrpPayload []byte) (int64, []byte, error) {
	var (
		err             error
		payloadResponse []byte
		statusCode      = int64(520)
	)

	if len(wrpPayload) == 0 {
		return statusCode, []byte(fmt.Sprintf(`{"message": ""Invalid Input Command"", "statusCode": %d}`, statusCode)), nil
	}

	payload := new(Tr181Payload)
	err = json.Unmarshal(wrpPayload, &payload)
	if err != nil {
		return statusCode, payloadResponse, err
	}

	switch payload.Command {
	case "GET":
		return h.get(payload)
	case "SET":
		return h.set(payload)
	default:
		// currently only get and set are implemented for existing mocktr181
		return statusCode, []byte(fmt.Sprintf(`{"message": "command '%s' is not supported", "statusCode": %d}`, payload.Command, statusCode)), nil
	}
}

func (h Handler) get(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		Names:      tr181.Names,
		StatusCode: http.StatusOK,
	}

	var (
		failedNames    []string
		readableParams []Parameter
	)
	for _, name := range tr181.Names {
		var found bool
		for _, mockParameter := range h.parameters {
			if name == "" {
				continue
			}

			if !strings.HasPrefix(mockParameter.Name, name) {
				continue
			}

			// Check whether mockParameter is readable.
			if strings.Contains(mockParameter.Access, "r") {
				found = true
				readableParams = append(readableParams, Parameter{
					Name:       mockParameter.Name,
					Value:      mockParameter.Value,
					DataType:   mockParameter.DataType,
					Attributes: mockParameter.Attributes,
					Message:    "Success",
					Count:      1,
				})
				continue
			}

			// If the requested parameter is a wild card and is not readable,
			// then continue and don't count it as a failure.
			if name[len(name)-1] == '.' {
				continue
			}

			// mockParameter is not readable.
			failedNames = append(failedNames, mockParameter.Name)
		}

		if !found {
			// Requested parameter was not found.
			failedNames = append(failedNames, name)
		}
	}

	result.Parameters = readableParams
	// Check if any parameters failed.
	if len(failedNames) != 0 {
		// If any names failed, then do not return any parameters that succeeded.
		result.Parameters = []Parameter{{
			Message: fmt.Sprintf("Invalid parameter names: %s", failedNames),
		}}
		result.StatusCode = 520
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(result.StatusCode), payload, nil
}

func (h Handler) set(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		Names:      tr181.Names,
		StatusCode: http.StatusAccepted,
	}

	var (
		writableParams []*MockParameter
		failedParams   []Parameter
	)
	// Check for any parameters that are not writable.
	for _, parameter := range tr181.Parameters {
		var found bool
		for i := range h.parameters {
			mockParameter := &h.parameters[i]
			if mockParameter.Name != parameter.Name {
				continue
			}

			// Check whether mockParameter is writable.
			if strings.Contains(mockParameter.Access, "w") {
				found = true
				// Add mockParameter to the list of parameters to be updated.
				writableParams = append(writableParams, mockParameter)
				continue
			}

			// mockParameter is not writable.
			failedParams = append(failedParams, Parameter{
				Name:    mockParameter.Name,
				Message: "Parameter is not writable",
			})
		}

		if !found {
			// Requested parameter was not found.
			failedParams = append(failedParams, Parameter{
				Name:    parameter.Name,
				Message: "Invalid parameter name",
			})
		}
	}

	// Check if any parameters failed.
	if len(failedParams) != 0 {
		// If any parameter failed, then do not apply any changes to the parameters in writableParams.
		writableParams = nil
		result.Parameters = failedParams
		result.StatusCode = 520
	}

	// If all the selected parameters are writable, then update the parameters. Otherwise, do nothing.
	for _, parameter := range tr181.Parameters {
		// writableParams will be nil if any parameters failed (i.e.: were not writable).
		for _, mockParameter := range writableParams {
			if mockParameter.Name != parameter.Name {
				continue
			}

			mockParameter.Value = parameter.Value
			mockParameter.DataType = parameter.DataType
			mockParameter.Attributes = parameter.Attributes
			result.Parameters = append(result.Parameters, Parameter{
				Name:       mockParameter.Name,
				Value:      mockParameter.Value,
				DataType:   mockParameter.DataType,
				Attributes: mockParameter.Attributes,
				Message:    "Success",
			})
		}
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(result.StatusCode), payload, nil
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
