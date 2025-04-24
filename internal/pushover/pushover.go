package pushover

import (
	"time"

	"github.com/gregdel/pushover"
)

type PushoverClient struct {
	app *pushover.Pushover
}

func NewPushoverClient(apiKey string) *PushoverClient {
	return &PushoverClient{
		app: pushover.New(apiKey),
	}
}

type PushoverMessage struct {
	Title     string
	Message   string
	Timestamp int64
	Recipient string
}

func (c *PushoverClient) SendNotification(msg PushoverMessage) error {
	pushoverMsg := &pushover.Message{
		Message:   msg.Message,
		Title:     msg.Title,
		Priority:  pushover.PriorityNormal,
		Timestamp: msg.Timestamp,
		Retry:     60 * time.Second,
		Expire:    time.Hour,
		Sound:     pushover.SoundCosmic,
	}

	recipient := pushover.NewRecipient(msg.Recipient)
	_, err := c.app.SendMessage(pushoverMsg, recipient)
	return err
}
