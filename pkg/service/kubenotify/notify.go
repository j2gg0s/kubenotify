package kubenotify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type NotifyFunc func(Event) error

func WebhookNotify(webhook string) NotifyFunc {
	return func(event Event) error {
		if b, err := json.Marshal(event); err != nil {
			return fmt.Errorf("marshal event with error: %w", err)
		} else if resp, err := http.Post(webhook, "application/json", bytes.NewBuffer(b)); err != nil {
			return fmt.Errorf("post %s with error: %w", webhook, err)
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("post %s without ok: %d", webhook, resp.StatusCode)
		}
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
