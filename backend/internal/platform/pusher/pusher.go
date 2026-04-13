// Package pusher provides a Pusher-compatible WebSocket client for real-time events.
package pusher

import (
	"fmt"
	"log/slog"

	pushersdk "github.com/pusher/pusher-http-go/v5"
)

// Client wraps the Pusher HTTP client.
type Client struct {
	client *pushersdk.Client
}

// Config holds Pusher connection parameters.
type Config struct {
	AppID   string
	Key     string
	Secret  string
	Host    string
	Cluster string
}

// New creates a new Pusher client.
func New(cfg Config) (*Client, error) {
	client := &pushersdk.Client{
		AppID:   cfg.AppID,
		Key:     cfg.Key,
		Secret:  cfg.Secret,
		Cluster: cfg.Cluster,
	}

	// If a custom host is specified (e.g., Soketi for local dev), use it.
	if cfg.Host != "" {
		client.Host = cfg.Host
		client.Secure = false // local dev doesn't use TLS
	}

	slog.Info("pusher client created", "app_id", cfg.AppID, "host", cfg.Host)
	return &Client{client: client}, nil
}

// Trigger sends an event on the given channel.
func (c *Client) Trigger(channel, event string, data interface{}) error {
	err := c.client.Trigger(channel, event, data)
	if err != nil {
		return fmt.Errorf("trigger %s/%s: %w", channel, event, err)
	}
	return nil
}

// TriggerBatch sends multiple events at once.
func (c *Client) TriggerBatch(events []pushersdk.Event) error {
	_, err := c.client.TriggerBatch(events)
	if err != nil {
		return fmt.Errorf("trigger batch: %w", err)
	}
	return nil
}

// AuthenticatePrivateChannel signs a private channel subscription request.
// params is the raw application/x-www-form-urlencoded request body containing
// socket_id and channel_name as sent by the Pusher JS client.
func (c *Client) AuthenticatePrivateChannel(params []byte) ([]byte, error) {
	return c.client.AuthenticatePrivateChannel(params)
}
