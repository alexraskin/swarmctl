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
	Message string
	Title   string
}

func (c *PushoverClient) SendNotification(msg PushoverMessage) error {
	pushoverMsg := &pushover.Message{
		Message:   msg.Message,
		Title:     msg.Title,
		Priority:  pushover.PriorityEmergency,
		Timestamp: time.Now().Unix(),
		Retry:     60 * time.Second,
		Expire:    time.Hour,
		Sound:     pushover.SoundCosmic,
	}

	recipient := pushover.NewRecipient(c.cfg.PushoverRecipient)
	_, err := c.app.SendMessage(pushoverMsg, recipient)
	return err
}
