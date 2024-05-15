// SPDX-FileCopyrightText: 2023 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/pem"
	"os"
	"strings"

	"github.com/xmidt-org/wrp-go/v3"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt"
	"github.com/xmidt-org/xmidt-agent/internal/jwtxt/event"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type instructionsIn struct {
	fx.In
	Service XmidtService
	ID      Identity
	Logger  *zap.Logger
}
type instructionsOut struct {
	fx.Out
	JWTXT     *jwtxt.Instructions
	DeviceID  wrp.DeviceID
	PartnerID string
}

func provideInstructions(in instructionsIn) (instructionsOut, error) {
	// If no PEMs are provided then the jwtxt can't be used because it won't
	// have any keys to use.
	if in.Service.URL == "" ||
		(in.Service.JwtTxtRedirector.PEMFiles == nil && in.Service.JwtTxtRedirector.PEMs == nil) {
		// return with DeviceID since ID.DeviceID is required for other fx components
		return instructionsOut{DeviceID: in.ID.DeviceID}, nil
	}

	logger := in.Logger.Named("jwtxt")

	opts := []jwtxt.Option{
		jwtxt.BaseURL(in.Service.URL),
		jwtxt.DeviceID(string(in.ID.DeviceID)),
		jwtxt.Algorithms(in.Service.JwtTxtRedirector.AllowedAlgorithms...),
		jwtxt.Timeout(in.Service.JwtTxtRedirector.Timeout),
		jwtxt.WithFetchListener(event.FetchListenerFunc(
			func(fe event.Fetch) {
				logger.Debug("fetch",
					zap.String("fqdn", fe.FQDN),
					zap.String("server", fe.Server),
					zap.Bool("found", fe.Found),
					zap.Bool("timeout", fe.Timeout),
					zap.Time("at", fe.At),
					zap.Duration("duration", fe.Duration),
					zap.Time("prior_expiration", fe.PriorExpiration),
					zap.Time("expiration", fe.Expiration),
					zap.Bool("temporary_err", fe.TemporaryErr),
					zap.String("endpoint", fe.Endpoint),
					zap.ByteString("payload", fe.Payload),
					zap.Error(fe.Err),
				)
			})),
	}

	if len(in.Service.JwtTxtRedirector.PEMs) > 0 {
		pems := make([][]byte, 0, len(in.Service.JwtTxtRedirector.PEMs))
		for _, item := range in.Service.JwtTxtRedirector.PEMs {
			block, rest := pem.Decode([]byte(item))

			if block == nil || strings.TrimSpace(string(rest)) != "" {
				return instructionsOut{}, jwtxt.ErrInvalidInput
			}

			buf := pem.EncodeToMemory(block)
			if buf == nil {
				return instructionsOut{}, jwtxt.ErrInvalidInput
			}

			pems = append(pems, buf)
		}
		opts = append(opts, jwtxt.WithPEMs(pems...))
	}

	if len(in.Service.JwtTxtRedirector.PEMFiles) > 0 {
		for _, pemFile := range in.Service.JwtTxtRedirector.PEMFiles {
			data, err := os.ReadFile(pemFile)
			if err != nil {
				return instructionsOut{}, err
			}
			opts = append(opts, jwtxt.WithPEMs(data))
		}
	}

	jwtxt, err := jwtxt.New(opts...)

	return instructionsOut{
		JWTXT:     jwtxt,
		DeviceID:  in.ID.DeviceID,
		PartnerID: in.ID.PartnerID}, err
}
