package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/selamawitsh/iqub-platform/services/member-service/internal/application"
)

const (
	// ExchangeName is the single topic exchange for all domain events
	// (SRS section 5.3). The routing key is the event type.
	ExchangeName   = "iqub.events"
	publishTimeout = 5 * time.Second
	dialTimeout    = 5 * time.Second
)

// RabbitMQPublisher implements application.EventPublisher.
//
// It keeps one connection and one channel in confirm mode. If the broker is
// unreachable, Publish returns an error and the next call tries to reconnect,
// so a broker restart heals itself without restarting the service.
type RabbitMQPublisher struct {
	url    string
	logger *slog.Logger

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

var _ application.EventPublisher = (*RabbitMQPublisher)(nil)

// NewRabbitMQPublisher never fails: it tries to connect once so problems show
// up in the log at startup, and otherwise connects lazily on the first Publish.
// Registration must keep working while the broker is down (ADR-005).
func NewRabbitMQPublisher(url string, logger *slog.Logger) *RabbitMQPublisher {
	p := &RabbitMQPublisher{url: url, logger: logger}

	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ensureConnectedLocked(); err != nil {
		logger.Warn("rabbitmq not reachable at startup; will retry on publish",
			slog.String("error", err.Error()))
	}
	return p
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, e application.Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	// The event must still be published if the HTTP client disconnects right
	// after the database commit, so request cancellation is detached here.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), publishTimeout)
	defer cancel()

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := p.ensureConnectedLocked(); err != nil {
		return fmt.Errorf("connect to rabbitmq: %w", err)
	}

	confirmation, err := p.ch.PublishWithDeferredConfirmWithContext(
		ctx,
		ExchangeName,
		e.EventType, // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    e.EventID,
			Type:         e.EventType,
			Timestamp:    e.OccurredAt,
			Body:         body,
		},
	)
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("publish: %w", err)
	}

	acked, err := confirmation.WaitContext(ctx)
	if err != nil {
		p.closeLocked()
		return fmt.Errorf("wait for broker confirm: %w", err)
	}
	if !acked {
		return errors.New("broker rejected the message (nack)")
	}
	return nil
}

// Close releases the channel and connection (called on shutdown).
func (p *RabbitMQPublisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *RabbitMQPublisher) ensureConnectedLocked() error {
	if p.conn != nil && !p.conn.IsClosed() && p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	p.closeLocked()

	conn, err := amqp.DialConfig(p.url, amqp.Config{
		Heartbeat: 10 * time.Second,
		Dial:      amqp.DefaultDial(dialTimeout),
	})
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return err
	}

	// Publisher confirms: the broker acknowledges every message it accepted.
	if err := ch.Confirm(false); err != nil {
		_ = conn.Close()
		return err
	}

	// Durable topic exchange; declaring again with the same settings is a no-op.
	if err := ch.ExchangeDeclare(ExchangeName, "topic", true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return err
	}

	p.conn, p.ch = conn, ch
	p.logger.Info("connected to rabbitmq", slog.String("exchange", ExchangeName))
	return nil
}

func (p *RabbitMQPublisher) closeLocked() {
	if p.ch != nil {
		_ = p.ch.Close()
		p.ch = nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
		p.conn = nil
	}
}
