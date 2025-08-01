// SPDX-FileCopyrightText: 2024 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package mocktr181

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/xmidt-org/wrp-go/v5"
	"github.com/xmidt-org/xmidt-agent/internal/wrpkit"
)

// Constants for TR-181 parameter names that are used multiple times
const (
	// App management command base path
	appMgmtBasePath = "Device.X_COM_NOS_APP_MGMT."

	// App management commands
	appMgmtUninstallApps = appMgmtBasePath + "UninstallApps"
	appMgmtInstallApps   = appMgmtBasePath + "InstallApps"
	appMgmtClearCache    = appMgmtBasePath + "ClearCache"
	appMgmtClearData     = appMgmtBasePath + "ClearData"
	appMgmtLaunch        = appMgmtBasePath + "Launch"

	// Apps data path and parameters
	appsBasePath     = "Device.X_NOS_COM_APPS."
	numberOfAppsPath = appsBasePath + "NumberOfApps"

	// TR-181 commands
	cmdGet           = "GET"
	cmdSet           = "SET"
	cmdGetAttributes = "GET_ATTRIBUTES"
	cmdSetAttributes = "SET_ATTRIBUTES"
	cmdReplaceRows   = "REPLACE_ROWS"
	cmdDeleteRow     = "DELETE_ROW"
	cmdAddRow        = "ADD_ROW"

	// Common error messages
	msgPackageNotFound       = "Package not found"
	msgNoPackagesSpecified   = "No packages specified"
	msgInvalidParameterName  = "Invalid parameter name"
	msgParameterNotWritable  = "Parameter is not writable"
	msgInvalidValueType      = "not a string or string array"
	msgNoAttributesSpecified = "no attributes specified for GET_ATTRIBUTES command"

	// Common success messages
	msgSuccess = "Success"
	msgDeleted = "Deleted"
	msgCreated = "Created"

	// HTTP status codes
	statusTR181Error = 520 // TR-181 specific error status
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
	Value      interface{}
	Access     string
	DataType   int // add json labels here
	Attributes map[string]interface{}
	Delay      int
}

type MockParameters struct {
	Parameters []MockParameter
}

type Tr181Payload struct {
	Command    string                            `json:"command"`
	Names      []string                          `json:"names"`
	Parameters []Parameter                       `json:"parameters"`
	StatusCode int                               `json:"statusCode"`
	Table      string                            `json:"table,omitempty"`      // For REPLACE_ROWS and ADD_ROW commands
	Rows       map[string]map[string]interface{} `json:"rows,omitempty"`       // For REPLACE_ROWS command
	Row        interface{}                       `json:"row,omitempty"`        // For DELETE_ROW (string) or ADD_ROW (object)
	Attributes interface{}                       `json:"attributes,omitempty"` // For GET_ATTRIBUTES and SET_ATTRIBUTES commands - can be string or []string
}

type Parameters struct {
	Parameters []Parameter
}

type Parameter struct {
	Name       string                 `json:"name"`
	Value      interface{}            `json:"value"`
	DataType   int                    `json:"dataType"`
	Attributes map[string]interface{} `json:"attributes"`
	Message    string                 `json:"message"`
	Count      int                    `json:"parameterCount"`
}

type InstallApp struct {
	UUID        string `json:"UUID"`
	Location    string `json:"Location"`
	Version     string `json:"Version"`
	PackageName string `json:"PackageName"`
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
func (h *Handler) HandleWrp(msg wrp.Message) error {
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

func (h *Handler) proccessCommand(wrpPayload []byte) (int64, []byte, error) {
	var (
		err             error
		payloadResponse []byte
		statusCode      = int64(statusTR181Error)
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
	case cmdGet:
		return h.get(payload)
	case cmdSet:
		return h.set(payload)
	case cmdGetAttributes:
		return h.getAttributes(payload)
	case cmdSetAttributes:
		return h.setAttributes(payload)
	case cmdReplaceRows:
		return h.updateTableRow(payload)
	case cmdDeleteRow:
		return h.deleteTableRow(payload)
	case cmdAddRow:
		return h.createTableRow(payload)
	default:
		return statusCode, []byte(fmt.Sprintf(`{"message": "command '%s' is not supported", "statusCode": %d}`, payload.Command, statusCode)), nil
	}
}

func (h *Handler) get(tr181 *Tr181Payload) (int64, []byte, error) {
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
		matches, found := h.findMatchingParameters(name)

		if !found {
			// Requested parameter was not found.
			failedNames = append(failedNames, name)
			continue
		}

		for _, mockParameter := range matches {
			readable, shouldSkip := h.isParameterReadable(mockParameter, name)

			if readable {
				readableParams = append(readableParams, Parameter{
					Name:       mockParameter.Name,
					Value:      mockParameter.Value,
					DataType:   mockParameter.DataType,
					Attributes: mockParameter.Attributes,
					Message:    msgSuccess,
					Count:      1,
				})
			} else if !shouldSkip {
				// mockParameter is not readable and should be counted as failure
				failedNames = append(failedNames, mockParameter.Name)
			}
		}
	}

	result.Parameters = readableParams
	// Check if any parameters failed.
	if len(failedNames) != 0 {
		result = h.buildErrorResponse(tr181.Command, tr181.Names, failedNames, false)
	}

	return h.marshalResponse(result)
}

func (h *Handler) getAttributes(tr181 *Tr181Payload) (int64, []byte, error) {
	// Parse and validate attributes
	attributes, err := h.parseAttributes(tr181.Attributes)
	if err != nil {
		result := Tr181Payload{
			Command:    tr181.Command,
			Names:      tr181.Names,
			StatusCode: statusTR181Error,
			Parameters: []Parameter{{Message: err.Error()}},
		}
		return h.marshalResponse(result)
	}

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
		matches, found := h.findMatchingParameters(name)

		if !found {
			failedNames = append(failedNames, name)
			continue
		}

		for _, mockParameter := range matches {
			readable, shouldSkip := h.isParameterReadable(mockParameter, name)

			if readable {
				param, success, paramFailure, attrFailure, invalidAttr := h.processParameterForAttributes(mockParameter, attributes)
				if success {
					readableParams = append(readableParams, param)
				} else if paramFailure {
					failedNames = append(failedNames, mockParameter.Name)
				} else if attrFailure {
					failedNames = append(failedNames, fmt.Sprintf("%s:%s", mockParameter.Name, invalidAttr))
					break
				}
			} else if !shouldSkip {
				failedNames = append(failedNames, mockParameter.Name)
			}
		}
	}

	result.Parameters = readableParams
	if len(failedNames) != 0 {
		hasAttributeErrors := false
		for _, name := range failedNames {
			if strings.Contains(name, ":") {
				hasAttributeErrors = true
				break
			}
		}
		result = h.buildErrorResponse(tr181.Command, tr181.Names, failedNames, hasAttributeErrors)
	}

	return h.marshalResponse(result)
}

func (h *Handler) set(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		Names:      tr181.Names,
		StatusCode: http.StatusAccepted,
	}
	anyFailure := false

	mgmtKeys := map[string]struct{}{
		appMgmtUninstallApps: {},
		appMgmtInstallApps:   {},
		appMgmtClearCache:    {},
		appMgmtClearData:     {},
		appMgmtLaunch:        {},
	}

	for _, param := range tr181.Parameters {
		foundParam, errorParam, shouldContinue := h.findWritableParameter(param.Name)
		if !shouldContinue {
			result.Parameters = append(result.Parameters, *errorParam)
			anyFailure = true
			result.StatusCode = statusTR181Error
			continue
		}

		if _, isMgmt := mgmtKeys[param.Name]; isMgmt {
			var params []Parameter
			var status int
			switch param.Name {
			case appMgmtUninstallApps:
				params, status = h.handleUninstallApps(param)
			case appMgmtInstallApps:
				params, status = h.handleInstallApps(param)
			case appMgmtClearCache:
				params, status = h.handleClearCache(param)
			case appMgmtClearData:
				params, status = h.handleClearData(param)
			case appMgmtLaunch:
				params, status = h.handleLaunch(param)
			}
			result.Parameters = append(result.Parameters, params...)
			if status != http.StatusOK {
				anyFailure = true
				result.StatusCode = status
			}
		} else {
			foundParam.Value = param.Value
			foundParam.DataType = param.DataType
			foundParam.Attributes = param.Attributes
			result.Parameters = append(result.Parameters, Parameter{
				Name:       foundParam.Name,
				Value:      foundParam.Value,
				DataType:   foundParam.DataType,
				Attributes: foundParam.Attributes,
				Message:    msgSuccess,
			})
		}
	}

	if !anyFailure {
		result.StatusCode = http.StatusOK
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload,
			errors.Join(ErrInvalidResponsePayload, err)
	}
	return int64(result.StatusCode), payload, nil
}

func (h *Handler) setAttributes(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		Names:      tr181.Names,
		StatusCode: http.StatusAccepted,
	}
	anyFailure := false

	for _, param := range tr181.Parameters {
		foundParam, errorParam, shouldContinue := h.findWritableParameter(param.Name)
		if !shouldContinue {
			result.Parameters = append(result.Parameters, *errorParam)
			anyFailure = true
			result.StatusCode = statusTR181Error
			continue
		}

		if foundParam.Attributes == nil {
			result.Parameters = append(result.Parameters, Parameter{
				Name:    foundParam.Name,
				Message: "Parameter does not support attributes",
			})
			anyFailure = true
			result.StatusCode = statusTR181Error
			continue
		}

		maps.Copy(foundParam.Attributes, param.Attributes)

		result.Parameters = append(result.Parameters, Parameter{
			Name:       foundParam.Name,
			Attributes: foundParam.Attributes,
			Message:    msgSuccess,
		})
	}

	if !anyFailure {
		result.StatusCode = http.StatusOK
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(
			ErrInvalidResponsePayload, err)
	}
	return int64(result.StatusCode), payload, nil
}

func (h *Handler) updateTableRow(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		StatusCode: http.StatusOK,
	}

	if tr181.Table == "" {
		result.StatusCode = statusTR181Error
		result.Parameters = []Parameter{{
			Message: "Table field is required for REPLACE_ROWS operation",
		}}
		payload, _ := json.Marshal(result)
		return int64(result.StatusCode), payload, nil
	}

	if len(tr181.Rows) == 0 {
		result.StatusCode = statusTR181Error
		result.Parameters = []Parameter{{
			Message: "Rows field is required for REPLACE_ROWS operation",
		}}
		payload, _ := json.Marshal(result)
		return int64(result.StatusCode), payload, nil
	}

	anyFailure := false
	var resultParams []Parameter

	for rowIndex, rowData := range tr181.Rows {
		for paramName, value := range rowData {
			param, success := h.updateSingleRowParameter(tr181.Table, rowIndex, paramName, value)
			resultParams = append(resultParams, param)
			if !success {
				anyFailure = true
				result.StatusCode = statusTR181Error
			}
		}
	}

	if !anyFailure {
		result.StatusCode = http.StatusOK
	}

	result.Parameters = resultParams

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(result.StatusCode), payload, nil
}

func (h *Handler) deleteTableRow(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		StatusCode: http.StatusOK,
	}

	rowPath, ok := tr181.Row.(string)
	if !ok || rowPath == "" {
		result.StatusCode = statusTR181Error
		result.Parameters = []Parameter{{
			Message: "Row field is required for DELETE_ROW operation and must be a string",
		}}
		payload, _ := json.Marshal(result)
		return int64(result.StatusCode), payload, nil
	}

	rowPrefix := rowPath
	if !strings.HasSuffix(rowPrefix, ".") {
		rowPrefix += "."
	}

	deletedParams := h.deleteParametersByPrefix([]string{rowPrefix})

	if len(deletedParams) == 0 {
		result.StatusCode = 404
		result.Parameters = []Parameter{{
			Message: "No matching table row found for deletion",
		}}
	} else {
		result.Parameters = deletedParams
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(result.StatusCode), payload, nil
}

func (h *Handler) createTableRow(tr181 *Tr181Payload) (int64, []byte, error) {
	result := Tr181Payload{
		Command:    tr181.Command,
		StatusCode: http.StatusOK,
	}

	tableName, rowParams, err := validateTableRowInput(tr181)
	if err != nil {
		result.StatusCode = statusTR181Error
		result.Parameters = []Parameter{{
			Message: err.Error(),
		}}
		payload, _ := json.Marshal(result)
		return int64(result.StatusCode), payload, nil
	}

	if !strings.HasSuffix(tableName, ".") {
		tableName += "."
	}

	nextIndex := h.findNextTableIndex(tableName)
	rowPrefix := fmt.Sprintf("%s%d.", tableName, nextIndex)

	var resultParams []Parameter

	for _, param := range rowParams {
		fullParamName := rowPrefix + param.Name

		newParam := MockParameter{
			Name:       fullParamName,
			Value:      param.Value,
			Access:     "rw",
			DataType:   param.DataType,
			Attributes: param.Attributes,
		}

		h.parameters = append(h.parameters, newParam)

		resultParams = append(resultParams, Parameter{
			Name:    newParam.Name,
			Value:   newParam.Value,
			Message: msgCreated,
		})
	}

	result.StatusCode = http.StatusOK
	result.Parameters = resultParams

	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}

	return int64(result.StatusCode), payload, nil
}

func (h *Handler) loadFile() ([]MockParameter, error) {
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

func (h *Handler) handleUninstallApps(param Parameter) ([]Parameter, int) {
	var pkgs []string
	switch v := param.Value.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				pkgs = append(pkgs, s)
			}
		}
	case []string:
		pkgs = v
	case string:
		if v != "" {
			pkgs = append(pkgs, v)
		}
	default:
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgInvalidValueType,
		}}, 520
	}
	if len(pkgs) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgNoPackagesSpecified,
		}}, 520
	}

	// If the first package isn't installed, return a single failure entry
	firstPkg := pkgs[0]
	indexSet := h.getIndexesForPackage(firstPkg)
	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgPackageNotFound,
		}}, 520
	}

	// Otherwise uninstall each and collect deletions
	var result []Parameter
	for _, pkg := range pkgs {
		result = append(result, h.uninstallAppByPackage(pkg)...)
	}
	return result, http.StatusOK
}

func (h *Handler) handleInstallApps(param Parameter) ([]Parameter, int) {
	var apps []InstallApp
	appsBytes, err := json.Marshal(param.Value)
	if err != nil {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: "Invalid InstallApps value: " + err.Error(),
		}}, 520
	}
	if err := json.Unmarshal(appsBytes, &apps); err != nil {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: "Invalid InstallApps value: " + err.Error(),
		}}, 520
	}
	if len(apps) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgNoPackagesSpecified,
		}}, 520
	}

	var result []Parameter
	for _, app := range apps {
		if app.PackageName == "" {
			result = append(result, Parameter{
				Name:    param.Name,
				Value:   param.Value,
				Message: "Missing PackageName for install",
			})
			continue
		}
		result = append(result, h.installAppByPackage(app)...)
	}
	return result, http.StatusOK
}

func (h *Handler) handleClearCache(param Parameter) ([]Parameter, int) {
	// Build a slice of package names from the incoming value
	var pkgs []string
	switch v := param.Value.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				pkgs = append(pkgs, s)
			}
		}
	case []string:
		pkgs = v
	case string:
		if v != "" {
			pkgs = append(pkgs, v)
		}
	default:
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgInvalidValueType,
		}}, 520
	}
	if len(pkgs) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgNoPackagesSpecified,
		}}, 520
	}

	first := pkgs[0]
	indexSet := h.getIndexesForPackage(first)
	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgPackageNotFound,
		}}, 520
	}

	var result []Parameter
	for _, pkg := range pkgs {
		result = append(result, h.clearCacheByPackage(pkg)...)
	}
	return result, http.StatusOK
}

func (h *Handler) handleClearData(param Parameter) ([]Parameter, int) {
	var pkgs []string
	switch v := param.Value.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				pkgs = append(pkgs, s)
			}
		}
	case []string:
		pkgs = v
	case string:
		if v != "" {
			pkgs = append(pkgs, v)
		}
	default:
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgInvalidValueType,
		}}, 520
	}
	if len(pkgs) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgNoPackagesSpecified,
		}}, 520
	}

	first := pkgs[0]
	indexSet := h.getIndexesForPackage(first)
	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: msgPackageNotFound,
		}}, 520
	}

	var result []Parameter
	for _, pkg := range pkgs {
		result = append(result, h.clearDataByPackage(pkg)...)
	}
	return result, http.StatusOK
}

func (h *Handler) handleLaunch(param Parameter) ([]Parameter, int) {
	pkg, ok := param.Value.(string)
	if !ok || pkg == "" {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: "Invalid Launch value: not a string",
		}}, 520
	}
	indexSet := h.getIndexesForPackage(pkg)
	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    param.Name,
			Value:   param.Value,
			Message: "Package not installed",
		}}, 520
	}
	return []Parameter{{
		Name:    param.Name,
		Value:   param.Value,
		Message: "Launch successful",
	}}, http.StatusOK
}

func (h *Handler) uninstallAppByPackage(pkg string) []Parameter {
	indexSet := h.getIndexesForPackage(pkg)
	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    pkg,
			Message: msgPackageNotFound,
		}}
	}

	var prefixes []string
	for idx := range indexSet {
		prefixes = append(prefixes, appsBasePath+idx+".")
	}

	deletions := h.deleteParametersByPrefix(prefixes)
	h.updateNumberOfApps(-len(indexSet))
	return deletions
}

func (h *Handler) installAppByPackage(app InstallApp) []Parameter {
	maxIdx := 0
	for _, mp := range h.parameters {
		if strings.HasPrefix(mp.Name, appsBasePath) {
			tail := strings.TrimPrefix(mp.Name, appsBasePath)
			parts := strings.SplitN(tail, ".", 2)
			if len(parts) < 2 {
				continue
			}
			if idx, err := strconv.Atoi(parts[0]); err == nil && idx > maxIdx {
				maxIdx = idx
			}
		}
	}
	newIdx := maxIdx + 1
	idxStr := fmt.Sprintf("%d", newIdx)

	params := []MockParameter{
		{
			Name:   appsBasePath + idxStr + ".Package",
			Value:  app.PackageName,
			Access: "r",
		},
		{
			Name:   appsBasePath + idxStr + ".Name",
			Value:  app.PackageName,
			Access: "r",
		},
		{
			Name:   appsBasePath + idxStr + ".UUID",
			Value:  app.UUID,
			Access: "r",
		},
		{
			Name:   appsBasePath + idxStr + ".Location",
			Value:  app.Location,
			Access: "r",
		},
		{
			Name:   appsBasePath + idxStr + ".Version",
			Value:  app.Version,
			Access: "r",
		},
	}

	h.parameters = append(h.parameters, params...)

	h.updateNumberOfApps(1)

	result := make([]Parameter, len(params))
	for i, mp := range params {
		result[i] = Parameter{
			Name:     mp.Name,
			Value:    mp.Value,
			DataType: mp.DataType,
			Message:  "Installed",
		}
	}
	return result
}

func (h *Handler) updateNumberOfApps(delta int) {
	for i := range h.parameters {
		if h.parameters[i].Name == numberOfAppsPath {
			n := 0
			switch v := h.parameters[i].Value.(type) {
			case int:
				n = v
			case float64:
				n = int(v)
			case string:
				if parsed, err := strconv.Atoi(v); err == nil {
					n = parsed
				}
			default:
				n = 0
			}
			n += delta
			if n < 0 {
				n = 0
			}
			h.parameters[i].Value = n
			return
		}
	}
	val := delta
	if val < 0 {
		val = 0
	}
	h.parameters = append(h.parameters, MockParameter{
		Name:   numberOfAppsPath,
		Value:  val,
		Access: "r",
	})
}

func (h *Handler) getIndexesForPackage(pkg string) map[string]struct{} {
	indexSet := make(map[string]struct{})
	for _, mp := range h.parameters {
		if !strings.HasPrefix(mp.Name, appsBasePath) || !strings.HasSuffix(mp.Name, ".Package") {
			continue
		}
		tail := strings.TrimPrefix(mp.Name, appsBasePath)
		parts := strings.SplitN(tail, ".", 2)
		if len(parts) == 2 && parts[1] == "Package" && mp.Value == pkg {
			indexSet[parts[0]] = struct{}{}
		}
	}
	return indexSet
}

func (h *Handler) deleteParametersByPrefix(prefixes []string) []Parameter {
	var deletedParams []Parameter
	var newParams []MockParameter

	for _, param := range h.parameters {
		shouldDelete := false
		for _, prefix := range prefixes {
			if strings.HasPrefix(param.Name, prefix) {
				shouldDelete = true
				break
			}
		}

		if shouldDelete {
			deletedParams = append(deletedParams, Parameter{
				Name:    param.Name,
				Message: msgDeleted,
			})
		} else {
			newParams = append(newParams, param)
		}
	}

	h.parameters = newParams
	return deletedParams
}

func (h *Handler) clearCacheByPackage(pkg string) []Parameter {
	indexSet := h.getIndexesForPackage(pkg)

	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    pkg,
			Message: msgPackageNotFound,
		}}
	}

	var cleared []Parameter
	for idx := range indexSet {
		cacheParamName := appsBasePath + idx + ".Cache"
		for i := range h.parameters {
			if h.parameters[i].Name == cacheParamName {
				h.parameters[i].Value = "" // Clear the cache
				cleared = append(cleared, Parameter{
					Name:    cacheParamName,
					Message: "Cache cleared",
				})
				break
			}
		}
	}
	return cleared
}

func (h *Handler) clearDataByPackage(pkg string) []Parameter {
	indexSet := h.getIndexesForPackage(pkg)

	if len(indexSet) == 0 {
		return []Parameter{{
			Name:    pkg,
			Message: msgPackageNotFound,
		}}
	}

	var cleared []Parameter
	for idx := range indexSet {
		dataParamName := appsBasePath + idx + ".Data"
		for i := range h.parameters {
			if h.parameters[i].Name == dataParamName {
				h.parameters[i].Value = ""
				cleared = append(cleared, Parameter{
					Name:    dataParamName,
					Message: "Data cleared",
				})
				break
			}
		}
	}
	return cleared
}

func (h *Handler) findMatchingParameters(name string) ([]*MockParameter, bool) {
	var matches []*MockParameter
	found := false

	for i := range h.parameters {
		mockParameter := &h.parameters[i]
		if name == "" {
			continue
		}

		if !strings.HasPrefix(mockParameter.Name, name) {
			continue
		}

		matches = append(matches, mockParameter)
		found = true
	}

	return matches, found
}

func (h *Handler) isParameterReadable(param *MockParameter, requestName string) (readable bool, shouldSkip bool) {
	if strings.Contains(param.Access, "r") {
		return true, false
	}

	if len(requestName) > 0 && requestName[len(requestName)-1] == '.' {
		return false, true
	}

	return false, false
}

func (h *Handler) buildErrorResponse(command string, names []string, failedNames []string, hasAttributeErrors bool) Tr181Payload {
	result := Tr181Payload{
		Command:    command,
		Names:      names,
		StatusCode: statusTR181Error,
	}

	var message string
	if hasAttributeErrors {
		message = fmt.Sprintf("Invalid attribute names: %s", failedNames)
	} else {
		message = fmt.Sprintf("Invalid parameter names: %s", failedNames)
	}

	result.Parameters = []Parameter{{
		Message: message,
	}}

	return result
}

func (h *Handler) findWritableParameter(paramName string) (*MockParameter, *Parameter, bool) {
	for i := range h.parameters {
		if h.parameters[i].Name == paramName {
			param := &h.parameters[i]

			if !strings.Contains(param.Access, "w") {
				return nil, &Parameter{
					Name:    param.Name,
					Message: msgParameterNotWritable,
				}, false
			}

			return param, nil, true
		}
	}

	return nil, &Parameter{
		Name:    paramName,
		Message: msgInvalidParameterName,
	}, false
}

func (h *Handler) marshalResponse(result Tr181Payload) (int64, []byte, error) {
	payload, err := json.Marshal(result)
	if err != nil {
		return http.StatusInternalServerError, payload, errors.Join(ErrInvalidResponsePayload, err)
	}
	return int64(result.StatusCode), payload, nil
}

func (h *Handler) parseAttributes(attributes interface{}) ([]string, error) {
	var result []string

	if attributes == nil {
		return nil, fmt.Errorf(msgNoAttributesSpecified)
	}

	switch v := attributes.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
	case []string:
		result = v
	case string:
		if v != "" {
			for _, attr := range strings.Split(v, ",") {
				attr = strings.TrimSpace(attr)
				if attr != "" {
					result = append(result, attr)
				}
			}
		}
	default:
		return nil, fmt.Errorf("invalid attributes format: must be string or array of strings")
	}

	if len(result) == 0 {
		return nil, fmt.Errorf(msgNoAttributesSpecified)
	}

	return result, nil
}

func (h *Handler) processParameterForAttributes(param *MockParameter, attributes []string) (Parameter, bool, bool, bool, string) {
	if param.Attributes == nil {
		return Parameter{}, false, true, false, ""
	}

	for _, attrName := range attributes {
		if _, exists := param.Attributes[attrName]; !exists {
			return Parameter{}, false, false, true, attrName
		}
	}

	requestedAttrs := make(map[string]interface{})
	for _, attrName := range attributes {
		requestedAttrs[attrName] = param.Attributes[attrName]
	}

	return Parameter{
		Name:       param.Name,
		Attributes: requestedAttrs,
		Message:    msgSuccess,
		Count:      len(requestedAttrs),
	}, true, false, false, ""
}

func parseIndexFromParameterName(paramName, basePath string) int {
	if !strings.HasPrefix(paramName, basePath) {
		return -1
	}

	remaining := strings.TrimPrefix(paramName, basePath)
	parts := strings.Split(remaining, ".")
	if len(parts) == 0 {
		return -1
	}

	if idx, err := strconv.Atoi(parts[0]); err == nil {
		return idx
	}
	return -1
}

func (h *Handler) updateSingleRowParameter(tableName, rowIndex, paramName string, value interface{}) (Parameter, bool) {
	fullParamName := fmt.Sprintf("%s%s.%s", tableName, rowIndex, paramName)

	for i := range h.parameters {
		p := &h.parameters[i]
		if p.Name == fullParamName {
			if !strings.Contains(p.Access, "w") {
				return Parameter{
					Name:    fullParamName,
					Message: msgParameterNotWritable,
				}, false
			}
			p.Value = value
			return Parameter{
				Name:    p.Name,
				Value:   p.Value,
				Message: msgSuccess,
			}, true
		}
	}

	return Parameter{
		Name:    fullParamName,
		Message: msgInvalidParameterName,
	}, false
}

func validateTableRowInput(tr181 *Tr181Payload) (string, []Parameter, error) {
	tableName := tr181.Table
	if tableName == "" {
		if len(tr181.Names) > 0 {
			tableName = tr181.Names[0]
		}
	}

	if tableName == "" {
		return "", nil, fmt.Errorf("Table name is required for ADD_ROW operation (use Table field)")
	}

	var rowParams []Parameter
	if tr181.Row != nil {
		if rowData, ok := tr181.Row.(map[string]interface{}); ok {
			for paramName, value := range rowData {
				rowParams = append(rowParams, Parameter{
					Name:     paramName,
					Value:    value,
					DataType: 0,
				})
			}
		}
	} else if len(tr181.Parameters) > 0 {
		rowParams = tr181.Parameters
	}

	if len(rowParams) == 0 {
		return "", nil, fmt.Errorf("Row data is required for ADD_ROW operation (use Row or Parameters field)")
	}

	return tableName, rowParams, nil
}

func (h *Handler) findNextTableIndex(tableName string) int {
	maxIndex := -1
	for _, param := range h.parameters {
		if strings.HasPrefix(param.Name, tableName) {
			if idx := parseIndexFromParameterName(param.Name, tableName); idx > maxIndex {
				maxIndex = idx
			}
		}
	}
	return maxIndex + 1
}
