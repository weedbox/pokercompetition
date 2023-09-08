package pokercompetition

import (
	"time"

	"github.com/nats-io/jsm.go"
	"github.com/nats-io/nats.go"
	"github.com/weedbox/pokerface/match"
)

type NativeQueueManager struct {
	nc    *nats.Conn
	jsctx nats.JetStreamContext
	jsmm  *jsm.Manager
}

func NewNativeQueueManager() match.QueueManager {

	nqm := &NativeQueueManager{}

	return nqm
}

func (nqm *NativeQueueManager) Connect() error {

	var nc *nats.Conn

	nc, err := nats.Connect(
		"nats://0.0.0.0:4222",
		nats.Name("CP_LIB_POKERCOMPETITION"),
		nats.PingInterval((5 * time.Second)),
		nats.MaxPingsOutstanding(3),
		nats.MaxReconnects(-1), // means will reconnect forever
		// nats.ReconnectHandler(func(c *nats.Conn) {
		// 	eb.logger.Warning("[eventbus#Connect] nats.ReconnectHandler reconnecting.")
		// 	err := eb.setConn(nc, cb)
		// 	if err != nil {
		// 		eb.logger.Error("[eventbus#Connect] nats.ReconnectHandler Error: ", err)
		// 	}
		// 	eb.logger.Warning("[eventbus#Connect] nats.ReconnectHandler reconnected.")
		// }),
		// nats.DisconnectHandler(func(c *nats.Conn) {
		// 	eb.logger.Warning("[eventbus#Connect] nats.DisconnectHandler disconnected.")
		// }),
	)
	if err != nil {
		return err
	}

	jsctx, err := nc.JetStream(
		nats.PublishAsyncMaxPending(1024000), // 一次最多能 publish 多少訊息
	)
	if err != nil {
		return err
	}

	jsmm, err := jsm.New(nc, jsm.WithTimeout(10*time.Second))
	if err != nil {
		return err
	}

	nqm.nc = nc
	nqm.jsctx = jsctx
	nqm.jsmm = jsmm

	// fmt.Println("[DEBUG] nats is connected: nats://0.0.0.0:4222")

	return nil
}

func (nqm *NativeQueueManager) Close() error {

	nqm.nc.Close()
	// nqm.server.Shutdown()
	// nqm.server.WaitForShutdown()

	return nil
}

func (nqm *NativeQueueManager) Conn() *nats.Conn {
	return nqm.nc
}

func (nqm *NativeQueueManager) AssertQueue(queueName string, subject string) (match.Queue, error) {

	q := match.NewQueue(nqm, queueName, subject)

	err := q.Destroy()
	if err != nil {
		return nil, err
	}

	err = q.Assert()
	if err != nil {
		return nil, err
	}

	return q, nil
}
