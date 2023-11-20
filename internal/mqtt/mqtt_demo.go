package mqtt

import (
	// "os"
	// "os/signal"
	// "syscall"
	"fmt"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
	"go.uber.org/zap"
)

func (s *MqttServer) RunDemo() {
	s.logger.Debug("in Run Demo")
	// sigs := make(chan os.Signal, 1)
	// done := make(chan bool, 1)
	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	// go func() {
	// 	<-sigs
	// 	done <- true
	// }()

	go func() {
		for {
			s.logger.Debug("about to publish")
			err := s.server.Publish("direct/publish", []byte("packet scheduled message"), false, 0)
			if err != nil {
				fmt.Println("error publishing to mqtt" + err.Error())
				s.logger.Error("error publishing to mqtt", zap.Error(err))
			}
			time.Sleep(2 * time.Second)
		}
	}()

	callback := func(cl *mqtt.Client, sub packets.Subscription, pk packets.Packet) {
		s.logger.Info("inline client received message from subscription", zap.String("client", cl.ID), zap.Int("subscriptionId", sub.Identifier), zap.String("topic", pk.TopicName),zap.String("payload", string(pk.Payload)))
	}

	go func() {
		for {
			s.logger.Debug("about to subscribe")
			err := s.server.Subscribe("direct/#", 1, callback)
			if err != nil {
				s.logger.Error("error subscribing to mqtt", zap.Error(err))
			}
			time.Sleep(2 * time.Second)
		}
	}()

	//<-done
}
