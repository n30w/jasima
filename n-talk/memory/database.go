package memory

import (
	"context"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseStore struct {
	// postgres://your_username:your_username@host:port/your_database_name?sslmode=disable

	url      string
	messages []Message
	mu       sync.Mutex
	db       *pgxpool.Pool
}

func NewDatabaseStore(ctx context.Context, url string) (*DatabaseStore, error) {
	// Initialize a connection to the database.

	db, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}

	defer db.Close()

	// Return the object.
	return &DatabaseStore{
		url:      url,
		db:       db,
		messages: make([]Message, 0),
	}, nil
}

func (d *DatabaseStore) SaveWithContext(ctx context.Context) func(role ChatRole, text string) error {
	return func(role ChatRole, text string) error {
		query := ""
		args := pgx.NamedArgs{
			"": "",
		}

		_, err := d.db.Exec(ctx, query, args)
		if err != nil {
			return err
		}

		return nil
	}
}

func (d *DatabaseStore) RetrieveWithContext(ctx context.Context) func(n int) ([]Message, error) {
	return func(n int) ([]Message, error) {
		// https://pkg.go.dev/github.com/jackc/pgx/v5#RowToStructByPos
		rows, _ := d.db.Query(ctx, "")
		messages, err := pgx.CollectRows(rows, pgx.RowToStructByName[Message])
		if err != nil {
			return nil, err
		}

		return messages, nil
	}
}
