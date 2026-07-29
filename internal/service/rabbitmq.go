package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitQueue struct {
	conn            *amqp.Connection
	publisher       *amqp.Channel
	queue           string
	deadLetterQueue string
	mu              sync.Mutex
}

type RabbitOrderMessage struct {
	OrderNo string `json:"order_no"`
	Retry   int    `json:"retry"`
}

type RabbitDeadLetterMessage struct {
	OrderNo  string    `json:"order_no"`
	Retry    int       `json:"retry"`
	Reason   string    `json:"reason"`
	FailedAt time.Time `json:"failed_at"`
}

func NewRabbitQueue(url, queue string) (*RabbitQueue, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	publisher, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := publisher.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		_ = publisher.Close()
		_ = conn.Close()
		return nil, err
	}
	deadLetterQueue := queue + ".dlq"
	if _, err := publisher.QueueDeclare(deadLetterQueue, true, false, false, false, nil); err != nil {
		_ = publisher.Close()
		_ = conn.Close()
		return nil, err
	}
	return &RabbitQueue{conn: conn, publisher: publisher, queue: queue, deadLetterQueue: deadLetterQueue}, nil
}

func (q *RabbitQueue) Publish(ctx context.Context, orderNo string) error {
	return q.PublishRetry(ctx, RabbitOrderMessage{OrderNo: orderNo})
}

func (q *RabbitQueue) PublishRetry(ctx context.Context, msg RabbitOrderMessage) error {
	body, err := encodeRabbitOrderMessage(msg)
	if err != nil {
		return err
	}
	return q.publish(ctx, q.queue, body)
}

func (q *RabbitQueue) PublishDeadLetter(ctx context.Context, msg RabbitOrderMessage, reason string) error {
	body, err := json.Marshal(RabbitDeadLetterMessage{
		OrderNo:  msg.OrderNo,
		Retry:    msg.Retry,
		Reason:   reason,
		FailedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	return q.publish(ctx, q.deadLetterQueue, body)
}

func (q *RabbitQueue) publish(ctx context.Context, queue string, body []byte) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.publisher.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}

func (q *RabbitQueue) Consume() (<-chan amqp.Delivery, error) {
	consumer, err := q.conn.Channel()
	if err != nil {
		return nil, err
	}
	if _, err := consumer.QueueDeclare(q.queue, true, false, false, false, nil); err != nil {
		_ = consumer.Close()
		return nil, err
	}
	if err := consumer.Qos(8, 0, false); err != nil {
		_ = consumer.Close()
		return nil, err
	}
	deliveries, err := consumer.Consume(q.queue, "", false, false, false, false, nil)
	if err != nil {
		_ = consumer.Close()
		return nil, err
	}
	return deliveries, nil
}

func (q *RabbitQueue) Close() error {
	_ = q.publisher.Close()
	return q.conn.Close()
}

func encodeRabbitOrderMessage(msg RabbitOrderMessage) ([]byte, error) {
	if msg.OrderNo == "" {
		return nil, errors.New("rabbit order message requires order_no")
	}
	return json.Marshal(msg)
}

func decodeRabbitOrderMessage(body []byte) (RabbitOrderMessage, error) {
	var msg RabbitOrderMessage
	if err := json.Unmarshal(body, &msg); err == nil && msg.OrderNo != "" {
		if msg.Retry < 0 {
			msg.Retry = 0
		}
		return msg, nil
	}
	orderNo := strings.TrimSpace(string(body))
	if orderNo == "" {
		return RabbitOrderMessage{}, errors.New("empty rabbit order message")
	}
	return RabbitOrderMessage{OrderNo: orderNo}, nil
}
