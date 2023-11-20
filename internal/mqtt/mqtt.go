package mqtt

import (
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"go.uber.org/zap"
)

type MqttServer struct {
	server *mqtt.Server
	logger *zap.Logger
}

func New(logger *zap.Logger) (*MqttServer, error) {

	// TODO - refine and configure
	server := mqtt.New(&mqtt.Options{
		Capabilities: &mqtt.Capabilities{
		  MaximumSessionExpiryInterval: 3600,
		  Compatibilities: mqtt.Compatibilities{
			ObscureNotAuthorized: true,
		  },
		},
		ClientNetWriteBufferSize: 4096,
		ClientNetReadBufferSize: 4096,
		SysTopicResendInterval: 10,
		InlineClient: true,
	  })

	  // allow all connections
	  err := server.AddHook(new(auth.AllowHook), nil)
	  if (err != nil) {
		logger.Error("error adding auth hook to mqtt server", zap.Error(err))
		return nil, err
	  }

	  // Create a TCP listener on a standard port.
      tcp := listeners.NewTCP("t1", ":1883", nil)
	  err = server.AddListener(tcp)
	  if err != nil {
		logger.Error("error creating tcp listener", zap.Error(err))
		return nil, err
	  }

	  return &MqttServer{
		server: server,
		logger: logger,
	  }, nil
}

func (s *MqttServer) Run() {
	// Create signals channel to run server until interrupted
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
	  <-sigs
	  done <- true
	}()

	go func() {
		s.logger.Debug("about to start broker")
		err := s.server.Serve()
		if err != nil {
			s.logger.Fatal("failed to start mqtt server", zap.Error(err))  // TODO - best way to communicate fatal error from goroutine
		}
	}()
	
	
	<-done
}
