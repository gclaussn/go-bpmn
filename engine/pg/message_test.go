package pg

import (
	"context"
	"testing"
	"time"

	"github.com/gclaussn/go-bpmn/engine"
	"github.com/gclaussn/go-bpmn/engine/internal"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
)

// !keep in sync with engine/mem/message_test.go
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

func TestMessageRepository(t *testing.T) {
	assert := assert.New(t)

	e := mustCreateEngine(t)
	defer e.Shutdown()

	pgEngine := e.(*pgEngine)

	t.Run("concurrent insert with conflict", func(t *testing.T) {
		pgCtx1, cancel1, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Errorf("failed to acquire context: %v", err)
		}

		defer cancel1()

		pgCtx2, cancel2, err := pgEngine.acquire(context.Background())
		if err != nil {
			t.Errorf("failed to acquire context: %v", err)
		}

		defer cancel2()

		message1 := &internal.MessageEntity{
			CorrelationKey: "ck",
			CreatedAt:      pgCtx1.Time(),
			CreatedBy:      "test",
			ExpiresAt:      pgtype.Timestamp{Time: pgCtx1.Time().Add(time.Hour), Valid: true},
			Name:           "uk",
			UniqueKey:      pgtype.Text{String: "c", Valid: true},
		}

		message2 := &internal.MessageEntity{
			CorrelationKey: "ck",
			CreatedAt:      pgCtx2.Time(),
			CreatedBy:      "test",
			ExpiresAt:      pgtype.Timestamp{Time: pgCtx2.Time().Add(time.Hour), Valid: true},
			Name:           "uk",
			UniqueKey:      pgtype.Text{String: "c", Valid: true},
		}

		insert1Ctx, insert1Cancel := context.WithCancel(context.Background())
		insert2Ctx, insert2Cancel := context.WithCancel(context.Background())

		var insertErr1 error
		var insertErr2 error
		go func() {
			insertErr1 = pgCtx1.Messages().Insert(message1)
			insert1Cancel()

			insertErr2 = pgCtx2.Messages().Insert(message2)
			insert2Cancel()
		}()

		<-insert1Ctx.Done()

		if insertErr1 != nil {
			t.Errorf("failed to insert message 1: %v", insertErr1)
		}

		pgEngine.release(pgCtx1, insertErr1)

		<-insert2Ctx.Done()

		if insertErr2 != nil {
			t.Errorf("failed to insert message 2: %v", insertErr2)
		}

		pgEngine.release(pgCtx2, insertErr2)

		assert.True(message2.IsConflict)
		message2.IsConflict = false
		assert.Equal(message1, message2)

		if err := pgEngine.execute(func(pgCtx *pgContext) error {
			message, err := pgCtx.Messages().Select(message1.Id)

			assert.Equal(message1, message)

			return err
		}); err != nil {
			t.Errorf("failed to select message: %v", err)
		}
	})
}
