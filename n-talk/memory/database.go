package memory

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseStore struct {
	// postgres://your_username:your_username@host:port/your_database_name?sslmode=disable

	url      string
	messages []Message
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

func (d *DatabaseStore) Save(ctx context.Context, role ChatRole, text string) error {
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

func (d *DatabaseStore) Retrieve(ctx context.Context, n int) ([]Message, error) {
	// https://pkg.go.dev/github.com/jackc/pgx/v5#RowToStructByPos
	rows, _ := d.db.Query(ctx, "")
	messages, err := pgx.CollectRows(rows, pgx.RowToStructByName[Message])
	if err != nil {
		return nil, err
	}

	return messages, nil
}
