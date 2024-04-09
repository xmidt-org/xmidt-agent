// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: LicenseRef-COMCAST

package integrationTests

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/ksuid"
	"github.com/xmidt-org/wrp-go/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TALARIA_ENDPOINT = "http://localhost:6200/device/send"
const SECRET = "secret"

// testFlags returns "" to run no tests, "all" to run all tests, "broken" to
// run only broken tests, and "working" to run only working tests.
func testFlags() string {
	env := os.Getenv("INTEGRATION_TESTS_RUN")
	env = strings.ToLower(env)
	env = strings.TrimSpace(env)

	switch env {
	case "all":
	case "broken":
	case "":
	default:
		return "working"
	}

	return env
}

func postWrpEvent(event wrp.Message) ([]byte, error) {
	encodedBytes := []byte{}

	encoder := wrp.NewEncoderBytes(&encodedBytes, wrp.Msgpack)
	if err := encoder.Encode(event); err != nil {
		return nil, err
	}

	return callEndpoint(TALARIA_ENDPOINT, encodedBytes)
}

func callEndpoint(url string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		fmt.Print(err.Error())
		return nil, err
	}

	h := hmac.New(sha1.New, []byte(SECRET))
	h.Write(body)
	signature := fmt.Sprintf("sha1=%x", h.Sum(nil))
	req.Header.Set("X-Webpa-Signature", signature)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("client: error making http request: %s\n", err)
		return nil, err
	}

	responseData, err := io.ReadAll(res.Body) // TODO
	if err != nil {
		return nil, err
	}

	return responseData, nil
}

type aTest struct {
	broken   bool
	msg      wrp.Message
	validate func(wrp.Message) error
}

func runIt(t *testing.T, tc aTest) {
	assert := assert.New(t)
	require := require.New(t)

	switch testFlags() {
	case "":
		t.Skip("skipping integration test")
	case "all":
	case "broken":
		if !tc.broken {
			t.Skip("skipping non-broken integration test")
		}
	default: // Including working
		if tc.broken {
			t.Skip("skipping broken integration test")
		}
	}

	// To avoid running tests in parallel, set `test.parallel` to `1`
	t.Parallel()

	ksession, err := ksuid.NewRandomWithTime(time.Now())
	require.NoError(err)
	sessionID := ksession.String()

	tc.msg.SessionID = sessionID

	response, err := postWrpEvent(tc.msg)

	var message wrp.Message
	payloadFormat := wrp.Msgpack
	err = wrp.NewDecoderBytes(response, payloadFormat).Decode(&message)

	tc.validate(message)

	assert.NoError(err)

	assert.True(true)
}

func timeOffset(t time.Time, offset string) time.Time {
	d, err := time.ParseDuration(offset)
	if err != nil {
		panic(err)
	}
	return t.Add(d)
}

func envDefault(name, def string) string {
	s := strings.TrimSpace(os.Getenv(name))
	if s == "" {
		return def
	}
	return s
}
