package memory

import (
	"context"
	"database/sql"
	"sync"

	_ "github.com/lib/pq"
)

type DatabaseStore struct {
	// postgres://your_username:your_username@host:port/your_database_name?sslmode=disable

	url      string
	messages []Message
	mu       sync.Mutex
	db       *sql.DB
}

func NewDatabaseStore(ctx context.Context, url string) (*DatabaseStore, error) {

	// Initialize a connection to the database.

	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}

	defer db.Close()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	// Return the object.
	return &DatabaseStore{
		url:      url,
		db:       db,
		messages: make([]Message, 0),
	}, nil
}

func (d *DatabaseStore) Save(role ChatRole, text string) error {
	return nil
}

func (d *DatabaseStore) Retrieve(n int) ([]Message, error) {
	// Example query
	rows, err := d.db.Query("SELECT * FROM your_table")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Process query results
	for rows.Next() {
		// Process each row
	}
	return nil, nil
}
