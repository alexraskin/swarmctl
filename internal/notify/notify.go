// Package notify delivers alerts through shoutrrr, so the destination is a
// user-supplied URL rather than a provider baked into swarmctl. Discord, Slack,
// Pushover, Telegram, ntfy, generic webhooks and everything else shoutrrr
// supports work without a code change.
package notify

import (
	"errors"
	"fmt"

	"github.com/containrrr/shoutrrr"
	"github.com/containrrr/shoutrrr/pkg/router"
	"github.com/containrrr/shoutrrr/pkg/types"
)

// Notifier fans a message out to every configured shoutrrr URL. A Notifier
// built from an empty URL list is disabled: Send is a no-op that reports
// success, so notifications stay optional.
type Notifier struct {
	router *router.ServiceRouter
	count  int
}

// New builds a Notifier for urls. Passing no URLs returns a disabled Notifier
// rather than an error. A malformed or unsupported URL is an error here, at
// startup, instead of a surprise the first time something crashes at 3am.
func New(urls []string) (*Notifier, error) {
	if len(urls) == 0 {
		return &Notifier{}, nil
	}

	sender, err := shoutrrr.CreateSender(urls...)
	if err != nil {
		return nil, fmt.Errorf("failed to create notification sender: %w", err)
	}

	return &Notifier{router: sender, count: len(urls)}, nil
}

// Enabled reports whether any destination is configured.
func (n *Notifier) Enabled() bool { return n.router != nil }

// Services returns the number of configured destinations.
func (n *Notifier) Services() int { return n.count }

// Send delivers message to every configured service and joins whatever failed.
// One dead destination does not stop the others: shoutrrr sends concurrently
// and returns a result per service.
func (n *Notifier) Send(title, message string) error {
	if !n.Enabled() {
		return nil
	}

	params := types.Params{"title": title}

	var failures []error
	for _, err := range n.router.Send(message, &params) {
		if err != nil {
			failures = append(failures, err)
		}
	}

	return errors.Join(failures...)
}
