package sync

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/go-pg/pg/v10"
	"github.com/rs/zerolog/log"

	"github.com/j2gg0s/kubenotify/pkg/model"
)

type Value struct {
	model.EmbeddedEvent
	Resource interface{}
}

type Key struct {
	Cluster   string
	Namespace string
	Name      string
	APIGroup  string
	Kind      string
}

func getEncoder(obj interface{}) sarama.Encoder {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(fmt.Errorf("marshal json with error: %v, %v", err, obj))
	}
	return sarama.ByteEncoder(b)
}

func SendMessage(
	ctx context.Context, producer sarama.AsyncProducer,
	cluster string, event Event,
) error {
	action := "add"
	if event.To == nil {
		action = "del"
	}

	obj := event.To
	typeMeta, objectMeta := GetMeta(obj)

	topic := fmt.Sprintf(
		"%s.%s.%s",
		cluster, objectMeta.Namespace, typeMeta.Kind)
	producer.Input() <- &sarama.ProducerMessage{
		Topic: topic,
		Key: getEncoder(Key{
			Cluster:   cluster,
			Namespace: objectMeta.Namespace,
			Name:      objectMeta.Name,
			Kind:      typeMeta.Kind,
		}),
		Value: getEncoder(Value{
			EmbeddedEvent: model.EmbeddedEvent{
				Cluster:         cluster,
				ResourceVersion: objectMeta.ResourceVersion,
				Namespace:       objectMeta.Namespace,
				Name:            objectMeta.Name,
				Kind:            typeMeta.Kind,
				Action:          action,
			},
			Resource: obj,
		}),
	}

	return nil
}

func NewMessageHandler(pgClient *pg.DB) func(context.Context, *sarama.ConsumerMessage) error {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		key := Key{}
		if err := json.Unmarshal(msg.Key, &key); err != nil {
			return fmt.Errorf("unmarshal key with error: %v, %s", err, string(msg.Key))
		}

		var event model.Event
		switch key.Kind {
		case "Deployment":
			event = &model.EventDeployment{
				EmbeddedEvent: &model.EmbeddedEvent{
					Cluster: key.Cluster,
				},
			}
		case "ConfigMap":
			event = &model.EventConfigMap{
				EmbeddedEvent: &model.EmbeddedEvent{
					Cluster: key.Cluster,
				},
			}
		default:
			log.Error().Msgf("ignore unsupport kind: %s", key.Kind)
			return nil
		}

		if err := json.Unmarshal(msg.Value, event); err != nil {
			log.Error().Msgf("unmarshal value with error: %s", string(msg.Value))
			return nil
		}

		if event.GetAction() == "add" {
			err := pgClient.ModelContext(ctx, event).WherePK().AllWithDeleted().Select()
			if err == pg.ErrNoRows {
				if _, err := pgClient.ModelContext(ctx, event).Insert(); err != nil {
					return fmt.Errorf("pg insert with error: %s", err)
				}
				log.Error().Msgf("insert event: %s", event.GetIdentity())
				return nil
			}
			log.Debug().Msgf("ignore existed event: %s", event.GetIdentity())
		} else if event.GetAction() == "del" {
			_, err := pgClient.ModelContext(ctx, event).WherePK().Delete()
			if err != nil {
				return fmt.Errorf("pg delete with error: %s", err)
			}
			log.Debug().Msgf("delete event: %s", event.GetIdentity())
		} else {
			log.Debug().Interface("event", event).Msgf("unknown event")
		}
		return nil
	}
}
