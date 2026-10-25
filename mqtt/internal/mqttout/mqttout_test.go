package mqttout_test

import (
	"net"
	"strconv"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/mqttout"
)

// startBroker runs an in-process MQTT broker on a free local port.
func startBroker(t *testing.T) (*mochi.Server, int) {
	t.Helper()
	var lc net.ListenConfig
	l, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port //nolint:forcetypeassert // a TCP listener has a TCP address
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	server := mochi.New(&mochi.Options{InlineClient: true})
	if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
		t.Fatal(err)
	}
	if err := server.AddListener(listeners.NewTCP(listeners.Config{
		ID: "test", Address: "127.0.0.1:" + strconv.Itoa(port),
	})); err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve() }()
	t.Cleanup(func() { _ = server.Close() })
	return server, port
}

func waitSignal(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("no republish signal after %s", what)
	}
}

func TestConnectPublishAndRepublishSignals(t *testing.T) {
	t.Parallel()
	server, port := startBroker(t)
	topics := discovery.DefaultTopics()

	received := make(chan packets.Packet, 10)
	if err := server.Subscribe("lochinvar/#", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		received <- pk
	}); err != nil {
		t.Fatal(err)
	}

	c := mqttout.New(mqttout.Settings{Host: "127.0.0.1", Port: port, ClientID: "test-bridge"}, topics)
	c.Connect()
	t.Cleanup(func() { c.Close(100 * time.Millisecond) })
	waitSignal(t, c.Republish(), "connecting")

	// The last will marks the bridge offline if it drops off the broker.
	cl, ok := server.Clients.Get("test-bridge")
	if !ok {
		t.Fatal("bridge not connected to the broker")
	}
	will := cl.Properties.Will
	if will.TopicName != topics.Status() || string(will.Payload) != discovery.Offline || !will.Retain {
		t.Errorf("last will %s=%q retain=%v, want %s=offline retained", will.TopicName, will.Payload, will.Retain,
			topics.Status())
	}

	c.Publish("lochinvar/sensor/outlet_temp/state", "150.0", true)
	select {
	case pk := <-received:
		if pk.TopicName != "lochinvar/sensor/outlet_temp/state" || string(pk.Payload) != "150.0" {
			t.Errorf("broker got %s=%q", pk.TopicName, pk.Payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("published message never reached the broker")
	}

	// Home Assistant's birth message asks for discovery again. Give the subscription a moment.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := server.Publish(topics.HAStatus(), []byte(discovery.Online), false, 0); err != nil {
			t.Fatal(err)
		}
		select {
		case <-c.Republish():
			return
		case <-time.After(100 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			t.Fatal("no republish signal after Home Assistant came online")
		}
	}
}
