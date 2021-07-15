package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

type NotifyFunc func(string) error

func WebhookNotify(addr string) NotifyFunc {
	return func(msg string) error {
		return retry.OnError(
			retry.DefaultBackoff,
			func(err error) bool {
				log.Warn().Err(err).Send()
				return true
			},
			func() error {
				body, err := json.Marshal(map[string]string{"message": msg})
				if err != nil {
					return fmt.Errorf("json marshal: %w", err)
				}

				resp, err := http.Post(addr, "application/json", bytes.NewBuffer(body))
				if err != nil {
					return fmt.Errorf("post %s with error: %w", addr, err)
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("post %s without ok: %d", addr, resp.StatusCode)
				}

				return nil
			},
		)
	}
}

func WebhooksNotify(addrs []string) NotifyFunc {
	hooks := make([]NotifyFunc, len(addrs))
	for i, addr := range addrs {
		hooks[i] = WebhookNotify(addr)
	}

	return func(msg string) error {
		group := wait.Group{}
		for _, hook := range hooks {
			group.Start(func() {
				if err := hook(msg); err != nil {
					log.Warn().Err(err).Msgf("ignore notify %s", msg)
				}
			})
		}
		group.Wait()
		return nil
	}
}

func StdoutNotify() NotifyFunc {
	return func(msg string) error {
		fmt.Println(msg)
		return nil
	}
}
