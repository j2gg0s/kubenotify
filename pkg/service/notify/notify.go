package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
	"k8s.io/client-go/util/retry"
)

type NotifyFunc func(Event) error

func WebhookNotify(webhook string) NotifyFunc {
	return func(event Event) error {
		return retry.OnError(
			retry.DefaultBackoff,
			func(error) bool { return true },
			func() error {
				if b, err := json.Marshal(event); err != nil {
					return fmt.Errorf("marshal event with error: %w", err)
				} else if resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(b)); err != nil {
					return fmt.Errorf("post %s with error: %w", webhook, err)
				} else if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("post %s without ok: %d", webhook, resp.StatusCode)
				}
				return nil
			},
		)
	}
}

func WebhooksNotify(webhooks []string) NotifyFunc {
	hooks := make([]NotifyFunc, len(webhooks))
	for i, webhook := range webhooks {
		hooks[i] = WebhookNotify(webhook)
	}

	return func(event Event) error {
		wg := sync.WaitGroup{}

		for i, h := range hooks {
			webhook := webhooks[i]
			hook := h
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := hook(event)
				if err != nil {
					log.Warn().Err(err).Msgf("failed to post webhook %s", webhook)
				}
			}()
		}

		wg.Wait()
		return nil
	}
}

func StdoutNotify() NotifyFunc {
	return func(event Event) error {
		if b, err := json.MarshalIndent(event, "", "  "); err != nil {
			return fmt.Errorf("marshal event with error: %w", err)
		} else {
			fmt.Fprintln(os.Stdout, string(b))
		}
		return nil
	}
}
