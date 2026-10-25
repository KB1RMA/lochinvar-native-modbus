// Package mqttout connects to the broker with a last will and signals when discovery and state must
// be republished: on every (re)connect and whenever Home Assistant comes online.
package mqttout

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/KB1RMA/lochinvar-native-modbus/mqtt/internal/discovery"
)

// Settings are the broker connection details.
type Settings struct {
	Host, Username, Password, ClientID string
	Port                               int
	TLS                                bool
	CAFile                             string // PEM file to verify the broker with instead of the system roots
}

// writeTimeout bounds each Publish. Without it paho waits up to 30 s per message for a stalled
// connection, which would hold up the Modbus poll loop.
const writeTimeout = 2 * time.Second

// Client publishes to the broker. Publish is safe for concurrent use.
type Client struct {
	client    paho.Client
	republish chan struct{}
}

// New prepares a client; call Connect to start it. It fails only when the CA file cannot be used.
func New(s Settings, topics discovery.Topics) (*Client, error) {
	c := &Client{republish: make(chan struct{}, 1)}
	var attempts atomic.Int64 // connection attempts since the last successful connect
	scheme := "tcp"
	if s.TLS {
		scheme = "ssl"
	}
	opts := paho.NewClientOptions().
		AddBroker(fmt.Sprintf("%s://%s:%d", scheme, s.Host, s.Port)).
		SetClientID(s.ClientID).
		SetUsername(s.Username).
		SetPassword(s.Password).
		SetWill(topics.Status(), discovery.Offline, 1, true).
		SetKeepAlive(60 * time.Second).
		SetWriteTimeout(writeTimeout).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetMaxReconnectInterval(time.Minute).
		SetCleanSession(true).
		SetConnectionAttemptHandler(func(_ *url.URL, cfg *tls.Config) *tls.Config {
			// Retries every 5 s give no error, so say so on the second attempt and then about once a minute.
			if n := attempts.Add(1); n == 2 || n%12 == 0 {
				slog.Warn("cannot reach MQTT broker yet; retrying", "host", s.Host, "port", s.Port, "attempts", n)
			}
			return cfg
		}).
		SetOnConnectHandler(func(cl paho.Client) {
			attempts.Store(0)
			slog.Info("connected to MQTT", "host", s.Host, "port", s.Port)
			cl.Subscribe(topics.HAStatus(), 0, func(_ paho.Client, m paho.Message) {
				if string(m.Payload()) == discovery.Online {
					c.signal()
				}
			})
			c.signal()
		}).
		SetConnectionLostHandler(func(_ paho.Client, err error) {
			slog.Warn("MQTT connection lost", "err", err)
		}).
		SetReconnectingHandler(func(_ paho.Client, _ *paho.ClientOptions) {
			slog.Info("reconnecting to MQTT", "host", s.Host, "port", s.Port)
		})
	if s.TLS {
		cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: s.Host}
		if s.CAFile != "" {
			pem, err := os.ReadFile(s.CAFile)
			if err != nil {
				return nil, fmt.Errorf("MQTT CA file: %w", err)
			}
			cfg.RootCAs = x509.NewCertPool()
			if !cfg.RootCAs.AppendCertsFromPEM(pem) {
				return nil, fmt.Errorf("MQTT CA file %s: no PEM certificates", s.CAFile)
			}
		}
		opts.SetTLSConfig(cfg)
	}
	c.client = paho.NewClient(opts)
	return c, nil
}

func (c *Client) signal() {
	select {
	case c.republish <- struct{}{}:
	default: // one pending signal is enough
	}
}

// Republish receives a value whenever everything should be published again.
func (c *Client) Republish() <-chan struct{} { return c.republish }

// Connect starts connecting in the background; with connect retry on it keeps trying until it succeeds.
func (c *Client) Connect() {
	c.client.Connect()
}

// Publish sends a QoS 0 message without waiting. Messages sent while disconnected are dropped; the
// bridge republishes everything on reconnect.
func (c *Client) Publish(topic, payload string, retain bool) {
	c.client.Publish(topic, 0, retain, payload)
}

// Close flushes for up to timeout and disconnects.
func (c *Client) Close(timeout time.Duration) {
	c.client.Disconnect(uint(timeout.Milliseconds())) //nolint:gosec // small positive timeout
}
