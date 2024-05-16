package main

import (
	"errors"

	"github.com/xmidt-org/xmidt-agent/internal/adapters/libparodus"
	"github.com/xmidt-org/xmidt-agent/internal/pubsub"
	"go.uber.org/fx"
)

type libParodusIn struct {
	fx.In

	// Configuration
	LibParodus LibParodus

	PubSub *pubsub.PubSub
}

func provideLibParodus(in libParodusIn) (*libparodus.Adapter, error) {
	libParodusDefaults := []libparodus.Option{
		libparodus.KeepaliveInterval(in.LibParodus.KeepAliveInterval),
		libparodus.ReceiveTimeout(in.LibParodus.ReceiveTimeout),
		libparodus.SendTimeout(in.LibParodus.SendTimeout),
	}
	libParodus, err := libparodus.New(in.LibParodus.ParodusServiceURL, in.PubSub, libParodusDefaults...)
	if err != nil {
		return nil, errors.Join(ErrWRPHandlerConfig, err)
	}

	return libParodus, nil
}
