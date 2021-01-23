package client

import "github.com/Shopify/sarama"

func NewKafkaProducer(addrs []string) (sarama.AsyncProducer, error) {
	return sarama.NewAsyncProducer(addrs, nil)
}
