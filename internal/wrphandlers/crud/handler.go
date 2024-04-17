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
	"github.com/xmidt-org/xmidt-agent/internal/loglevel"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

type Handler struct {
	egress     wrpkit.Handler
	source     string
	logLevelService *loglevel.LogLevelService
}


// New creates a new instance of the Handler struct.  The parameter egress is
// the handler that will be called to send the response.  The parameter source is the source to use in
// the response message.
func New(egress wrpkit.Handler, source string, logLevelService *loglevel.LogLevelService) (*Handler, error) {
	
	h := Handler{
		egress: egress,
		source: source,
		logLevelService:  logLevelService,
	}

	return &h, nil
}


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

func (h Handler) get(names []string) (int64, []byte, error) {
	result := Tr181Payload{}
	statusCode := int64(http.StatusOK)

	for _, name := range names {
		for _, mockParameter := range h.parameters {
			if strings.HasPrefix(mockParameter.Name, name) {
				result.Parameters = append(result.Parameters, Parameter{
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
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	if len(result.Parameters) == 0 {
		statusCode = int64(520)
	}

	return statusCode, payload, nil
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
