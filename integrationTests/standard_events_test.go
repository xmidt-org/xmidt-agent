// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: LicenseRef-COMCAST

package integrationTests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xmidt-org/wrp-go/v3"
)

// Send a single online event.
func TestGetParameters(t *testing.T) {
	assert := assert.New(t)
	//require := require.New(t)
	
	runIt(t, aTest{
		broken: false, 
		msg: wrp.Message{
			Type:        wrp.SimpleEventMessageType,
			Source:      "dns:tr1d1um.example.com/service/ignored",
			Destination: "event:event_1/ignored",
			Payload:     []byte("{\"command\":\"SET\",\"parameters\":[{\"name\":\"Device.WiFi.Radio.10000.Name\",\"dataType\":0,\"value\":\"anothername\",\"attributes\":{\"notify\":0}}]}"),
		},
		validate: func(msg wrp.Message) error {
			assert.Equal(int64(http.StatusAccepted), *msg.Status)

			return nil
		},
	})
}