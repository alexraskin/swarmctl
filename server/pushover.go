package server

import (
	"time"

	"github.com/gregdel/pushover"
)

type PushoverClient struct {
	app *pushover.Pushover
	cfg *Config
}

func NewPushoverClient(cfg *Config) *PushoverClient {
	return &PushoverClient{
		app: pushover.New(cfg.PushoverAPIKey),
		cfg: cfg,
	}
}

type PushoverMessage struct {
	Title     string
	Message   string
	Priority  int
	Timestamp int64
}

func (c *PushoverClient) SendNotification(msg PushoverMessage) error {
	pushoverMsg := &pushover.Message{
		Message:   msg.Message,
		Title:     msg.Title,
		Priority:  msg.Priority,
		Timestamp: msg.Timestamp,
		Retry:     60 * time.Second,
		Expire:    time.Hour,
		Sound:     pushover.SoundCosmic,
	}

	recipient := pushover.NewRecipient(c.cfg.PushoverRecipient)
	_, err := c.app.SendMessage(pushoverMsg, recipient)
	return err
}
