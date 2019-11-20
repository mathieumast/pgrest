package transactional_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aptogeo/pgrest/transactional"
	"github.com/go-pg/pg/v9"
	"github.com/go-pg/pg/v9/orm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

type Todo struct {
	ID   uuid.UUID `pg:",pk"`
	Text string
}

func (t *Todo) BeforeInsert(c context.Context) (context.Context, error) {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return c, nil
}

func initTests(t *testing.T) *pg.DB {
	db := pg.Connect(&pg.Options{
		User:               "postgres",
		Password:           "postgres",
		IdleCheckFrequency: 100 * time.Millisecond,
	})
	for _, model := range []interface{}{(*Todo)(nil)} {
		err := db.CreateTable(model, &orm.CreateTableOptions{
			Temp: true,
		})
		assert.Nil(t, err)
	}

	return db
}

func TestTransactionalCurrentKO(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ko"}
		tx.Insert(todo)
		return errors.New("ko")
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 0, count)
}

func TestTransactionalCurrentOK(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		return tx.Insert(todo)
	})
	assert.Nil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestTransactionalCurrentOKCurrentKO(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ko"}
			tx.Insert(todo)
			return errors.New("ko")
		})
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 0, count)
}

func TestTransactionalCurrentOKCurrentOK(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ok"}
			return tx.Insert(todo)
		})
	})
	assert.Nil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
}

func TestTransactionalMandatory(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Mandatory, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		return tx.Insert(todo)
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 0, count)

	err = transactional.Execute(ctx, func(ctx context.Context, tx *pg.Tx) error {
		return transactional.ExecuteWithPropagation(ctx, transactional.Current, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ok"}
			return tx.Insert(todo)
		})
	})
	assert.Nil(t, err)
	count, err = db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestTransactionalSavepointKO(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ko"}
		tx.Insert(todo)
		return errors.New("ko")
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 0, count)
}

func TestTransactionalSavepointOK(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ko"}
		return tx.Insert(todo)
	})
	assert.Nil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestTransactionalSavepointOKSavepointOK(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ok"}
			return tx.Insert(todo)
		})
	})
	assert.Nil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
}

func TestTransactionalSavepointOKSavepointKO(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ko"}
			tx.Insert(todo)
			return errors.New("ko")
		})
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestTransactionalCurrentOKSavepointKO(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Current, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ko"}
			tx.Insert(todo)
			return errors.New("ko")
		})
	})
	assert.NotNil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 1, count)
}

func TestTransactionalCurrentOKSavepointOK(t *testing.T) {
	db := initTests(t)
	var err error
	ctx := transactional.ContextWithDb(context.Background(), db)
	err = transactional.ExecuteWithPropagation(ctx, transactional.Current, func(ctx context.Context, tx *pg.Tx) error {
		todo := &Todo{Text: "ok"}
		err := tx.Insert(todo)
		if err != nil {
			return err
		}
		return transactional.ExecuteWithPropagation(ctx, transactional.Savepoint, func(ctx context.Context, tx *pg.Tx) error {
			todo := &Todo{Text: "ok"}
			return tx.Insert(todo)
		})
	})
	assert.Nil(t, err)
	count, err := db.Model(&Todo{}).Count()
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
}
