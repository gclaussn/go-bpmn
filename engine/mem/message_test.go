package mem

import (
	"context"
	"testing"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/pg/message_test.go
func TestSendMessage(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	t.Run("duplicate message is discarded", func(t *testing.T) {
		message1, err1 := e.SendMessage(context.Background(), engine.SendMessageCmd{
			CorrelationKey: "duplicate-message-ck",
			Name:           "duplicate-message",
			UniqueKey:      "duplicate-message-uk",
		})
		if err1 != nil {
			t.Errorf("failed to send message 1: %v", err1)
		}

		assert.NotNil(message1.ExpiresAt)
		assert.False(message1.IsCorrelated)

		message2, err2 := e.SendMessage(context.Background(), engine.SendMessageCmd{
			CorrelationKey: "duplicate-message-ck",
			Name:           "duplicate-message",
			UniqueKey:      "duplicate-message-uk",
		})
		if err2 != nil {
			t.Errorf("failed to send message 2: %v", err2)
		}

		assert.Equal(message1, message2)
	})

	t.Run("messages are buffered", func(t *testing.T) {
		message1, err1 := e.SendMessage(context.Background(), engine.SendMessageCmd{
			CorrelationKey: "buffered-message-ck",
			Name:           "buffered-message",
		})
		if err1 != nil {
			t.Errorf("failed to send message 1: %v", err1)
		}

		assert.NotNil(message1.ExpiresAt)
		assert.False(message1.IsCorrelated)

		message2, err2 := e.SendMessage(context.Background(), engine.SendMessageCmd{
			CorrelationKey: "buffered-message-ck",
			Name:           "buffered-message",
			UniqueKey:      "buffered-message-uk",
		})
		if err2 != nil {
			t.Errorf("failed to send message 2: %v", err2)
		}

		message3, err3 := e.SendMessage(context.Background(), engine.SendMessageCmd{
			CorrelationKey: "buffered-message-ck",
			Name:           "buffered-message",
		})
		if err3 != nil {
			t.Errorf("failed to send message 3: %v", err3)
		}

		assert.NotEqual(message1, message2)
		assert.NotEqual(message1, message3)
		assert.NotEqual(message2, message3)
	})
}
