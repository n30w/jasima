package memory

import (
	"context"
	"errors"
	"sync"
	"time"

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

func (d *DatabaseStore) Save(ctx context.Context, message Message) error {
	// ID of the sending agent.
	a1, err := d.getAgentId(ctx, message.Sender)
	if err != nil {
		return err
	}

	// ID of the receiving agent.
	a2, err := d.getAgentId(ctx, message.Receiver)
	if err != nil {
		return err
	}

	// ID of the agent who is inserting this row.
	a3, err := d.getAgentId(ctx, message.InsertedBy)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO messages (role, text, timestamp, sender_id, receiver_id, inserted_by)
		VALUES (@role, @text, @timestamp, @sender, @receiver, @inserted_by)
	`

	args := pgx.NamedArgs{
		"role":        message.Role,
		"text":        message.Text,
		"timestamp":   time.Now(),
		"sender":      a1,
		"receiver":    a2,
		"inserted_by": a3,
	}

	_, err = d.db.Exec(ctx, query, args)
	if err != nil {
		return err
	}

	return nil
}

// Retrieve retrieves all messages from the database and puts them in a slice
// of Messages. Check out pgx's query all rows with generic type.
// https://pkg.go.dev/github.com/jackc/pgx/v5#RowToStructByPos
func (d *DatabaseStore) Retrieve(ctx context.Context, name string, n int) ([]Message, error) {

	a1, err := d.getAgentId(ctx, name)
	if err != nil {
		return nil, err
	}

	query := `
		SELECT * FROM messages
		WHERE inserted_by = @inserted_by
	`

	args := pgx.NamedArgs{
		"inserted_by": a1,
	}

	rows, err := d.db.Query(ctx, query, args)
	if err != nil {
		return nil, err
	}

	messages, err := pgx.CollectRows(rows, pgx.RowToStructByName[Message])
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func (d *DatabaseStore) getAgentId(ctx context.Context, name string) (int, error) {
	query := `
		SELECT * FROM agents WHERE name = @name
	`

	args := pgx.NamedArgs{
		"name": name,
	}

	row := d.db.QueryRow(ctx, query, args)

	var agent struct {
		ID   int
		Name string
	}
	err := row.Scan(&agent.ID, &agent.Name)
	if err != nil {
		return -1, err
	}

	return agent.ID, nil
}

type InMemoryStore struct {
	messages []Message

	// total is the number of memories in the store
	total int

	mu sync.Mutex
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make([]Message, 0),
		total:    0,
	}
}

func (in *InMemoryStore) Save(_ context.Context, message Message) error {

	in.mu.Lock()
	in.messages = append(in.messages, message)
	in.mu.Unlock()

	in.total = len(in.messages)

	return nil
}

func (in *InMemoryStore) Retrieve(_ context.Context, _ string, n int) ([]Message, error) {

	in.mu.Lock()
	defer in.mu.Unlock()

	if n > in.total {
		return nil, errors.New("too many entries requested")
	}

	// Retrieve all memories
	if n <= 0 {
		return in.messages, nil
	}

	messages := make([]Message, n)

	for i := in.total - n; i < in.total; i++ {
		messages = append(messages, in.messages[i])
	}

	return messages, nil
}
