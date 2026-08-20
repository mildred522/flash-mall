package job

import (
	"context"
	"testing"

	"flash-mall/app/order/rpc/internal/config"
	"flash-mall/app/order/rpc/internal/svc"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestOutboxBatchSizeUsesConfiguredAndDefaultValues(t *testing.T) {
	publisher := &OutboxPublisher{svcCtx: &svc.ServiceContext{Config: config.Config{}}}
	if got := publisher.batchSize(); got != 200 {
		t.Fatalf("default batch size=%d", got)
	}
	publisher.svcCtx.Config.OutboxBatchSize = 64
	if got := publisher.batchSize(); got != 64 {
		t.Fatalf("configured batch size=%d", got)
	}
}

func TestRabbitPublisherAcceptsEmptyBatchWithoutConnection(t *testing.T) {
	publisher := NewRabbitPublisher("", "")
	if err := publisher.PublishBatch(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestMarkPublishedBatchUsesSingleStateUpdate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	publisher := &OutboxPublisher{svcCtx: &svc.ServiceContext{SqlConn: sqlx.NewSqlConnFromDB(db)}}
	mock.ExpectExec("(?s)UPDATE order_outbox.*id IN \\(\\?,\\?\\)").
		WithArgs(outboxStatusPublished, outboxStatusPublishing, int64(10), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 2))

	if err := publisher.markPublishedBatch(context.Background(), []outboxEvent{{ID: 10}, {ID: 11}}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestOutboxDrainContinuesOnlyAfterFullBatch(t *testing.T) {
	publisher := &OutboxPublisher{svcCtx: &svc.ServiceContext{Config: config.Config{OutboxBatchSize: 20}}}
	if publisher.batchWasFull(19) {
		t.Fatal("partial batch must return to idle polling")
	}
	if !publisher.batchWasFull(20) {
		t.Fatal("full batch must continue draining immediately")
	}
}
