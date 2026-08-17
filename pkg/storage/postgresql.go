package storage

import (
	"context"
	"fmt"
	"net/url"
	"os"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/rds/auth"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	log "github.com/sirupsen/logrus"

	"github.com/doctolib/MailHog/generated/queries"
	"github.com/doctolib/MailHog/pkg/data"
)

// PostgreSQL represents PostgreSQL backed storage backend
type PostgreSQL struct {
	Pool *pgxpool.Pool
}

// CreatePostgreSQL creates a PostgreSQL backed storage backend from a URI
func CreatePostgreSQL(uri string) *PostgreSQL {
	poolConfig, err := pgxpool.ParseConfig(uri)
	if err != nil {
		log.Errorf("Error parsing PostgreSQL URI: %s", err)
		os.Exit(1)
		return nil
	}
	return createPostgreSQL(poolConfig)
}

// CreatePostgreSQLFromParams builds the storage backend from the standard
// database env vars, optionally authenticating with AWS RDS IAM auth tokens
// instead of a static password.
func CreatePostgreSQLFromParams(host, port, name, user, region string, useIAM bool) *PostgreSQL {
	if port == "" {
		port = "5432"
	}
	dsn := fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=require", url.QueryEscape(user), host, port, name)
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		log.Errorf("Error building PostgreSQL config: %s", err)
		os.Exit(1)
		return nil
	}

	if useIAM {
		opts := []func(*awsconfig.LoadOptions) error{}
		if region != "" {
			opts = append(opts, awsconfig.WithRegion(region))
		}
		awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			log.Errorf("Error loading AWS config for RDS IAM auth: %s", err)
			os.Exit(1)
			return nil
		}
		endpoint := fmt.Sprintf("%s:%s", host, port)
		// RDS IAM auth tokens are short-lived (~15 min), so mint a fresh one for
		// every new connection the pool opens.
		poolConfig.BeforeConnect = func(ctx context.Context, connConfig *pgx.ConnConfig) error {
			token, err := auth.BuildAuthToken(ctx, endpoint, region, user, awsCfg.Credentials)
			if err != nil {
				return fmt.Errorf("building RDS IAM auth token: %w", err)
			}
			connConfig.Password = token
			return nil
		}
	}

	return createPostgreSQL(poolConfig)
}

func createPostgreSQL(poolConfig *pgxpool.Config) *PostgreSQL {
	log.Infof("Connecting to PostgreSQL: %s", poolConfig.ConnConfig.Host)
	pool, err := pgxpool.ConnectConfig(context.TODO(), poolConfig)
	if err != nil {
		log.Errorf("Error connecting to PostgreSQL: %s", err)
		// Do not fallback on in memory storage
		os.Exit(1)
		return nil
	}
	schema, err := queries.Asset("queries/postgresql-schema.sql")
	if err != nil {
		panic(err)
	}
	log.Infof("Creating or updating PostgreSQL schema.")
	pool.Exec(context.Background(), string(schema))

	return &PostgreSQL{
		Pool: pool,
	}
}

// Store stores a message in PostgreSQL and returns its storage ID
func (pg *PostgreSQL) Store(m *data.Message) (string, error) {
	log.Debugf("Storing message")
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return "", err
	}
	defer conn.Release()
	if _, err := conn.Exec(context.TODO(), "INSERT INTO messages(message) VALUES ($1)", m); err != nil {
		log.Errorf("Insert error %v", err)
		return "", err
	}
	return string(m.ID), nil
}

// Count returns the number of stored messages
func (pg *PostgreSQL) Count() int {
	log.Debugf("Counting")
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return -1
	}
	defer conn.Release()
	var n int
	if err := conn.QueryRow(context.TODO(), "SELECT count(*) FROM messages").Scan(&n); err != nil {
		log.Errorf("Count error %v", err)
		return -1
	}
	return n
}

// Search finds messages matching the query
func (pg *PostgreSQL) Search(kind, query string, start, limit int) (*data.Messages, int, error) {
	log.Debugf("Searching messages %s %s %d %d", kind, query, start, limit)
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return nil, 0, err
	}
	defer conn.Release()
	var field string
	switch kind {
	case SearchKindTo:
		field = "message->'Raw'->>'To'"
	case SearchKindFrom:
		field = "message->'Raw'->>'From'"
	case SearchKindContaining:
		field = "message->'Raw'->>'Data'"
	}
	sqlQuery := "SELECT message FROM messages WHERE to_tsvector('english', " + field + ") @@ to_tsquery('english', $1) ORDER BY message->'Created' DESC LIMIT $2 OFFSET $3"
	log.Tracef("Query: %s", sqlQuery)
	rows, err := conn.Query(context.TODO(), sqlQuery, query, limit, start)
	if err != nil {
		log.Errorf("Search error: %v", err)
		return nil, 0, err
	}
	defer rows.Close()
	messages := make([]data.Message, 0)
	for rows.Next() {
		var message data.Message
		if err = rows.Scan(&message); err != nil {
			log.Errorf("Error %v", err)
			return nil, 0, err
		}
		messages = append(messages, message)
	}
	msgs := data.Messages(messages)
	var count int
	if err := conn.QueryRow(context.TODO(), "SELECT count(*) FROM messages WHERE to_tsvector('english', "+field+") @@ to_tsquery('english', $1)", query).Scan(&count); err != nil {
		log.Errorf("Count error %v", err)
		return nil, 0, err
	}
	log.Printf("Query result: %d", count)
	return &msgs, count, nil
}

// List returns a list of messages by index
func (pg *PostgreSQL) List(start int, limit int) (*data.Messages, error) {
	// log.Printf("Listing messages %d %d", start, limit)
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	rows, err := conn.Query(context.TODO(), "SELECT message FROM messages ORDER BY message->'Created' DESC LIMIT $1 OFFSET $2", limit, start)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]data.Message, 0)
	for rows.Next() {
		var message data.Message
		if err = rows.Scan(&message); err != nil {
			log.Errorf("Error %v", err)
			return nil, err
		}
		messages = append(messages, message)
	}
	msgs := data.Messages(messages)
	return &msgs, nil
}

// DeleteOne deletes an individual message by storage ID
func (pg *PostgreSQL) DeleteOne(id string) error {
	log.Debugf("Deleting message %v", id)
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(context.TODO(), "DELETE FROM messages WHERE message->>'ID' = $1", id); err != nil {
		log.Errorf("Delete error %v", err)
		return err
	}
	return nil
}

// deleteAllBatchSize is the number of rows removed per iteration in DeleteAll.
// Declared as a var so tests can override it to exercise multi-batch behaviour
// without storing thousands of messages.
var deleteAllBatchSize = 1000

// DeleteAll deletes all messages stored in PostgreSQL in batches to avoid statement timeouts.
func (pg *PostgreSQL) DeleteAll() error {
	log.Debugf("Deleting all messages")
	for {
		conn, err := pg.Pool.Acquire(context.TODO())
		if err != nil {
			return err
		}
		tx, err := conn.BeginTx(context.TODO(), pgx.TxOptions{AccessMode: pgx.ReadWrite})
		if err != nil {
			conn.Release()
			return err
		}
		tag, err := tx.Exec(context.TODO(), "DELETE FROM messages WHERE ctid IN (SELECT ctid FROM messages LIMIT $1)", deleteAllBatchSize)
		if err != nil {
			tx.Rollback(context.TODO())
			conn.Release()
			log.Errorf("Delete error %v", err)
			return err
		}
		if err := tx.Commit(context.TODO()); err != nil {
			tx.Rollback(context.TODO())
			conn.Release()
			return err
		}
		conn.Release()
		if tag.RowsAffected() == 0 {
			break
		}
	}
	return nil
}

// Load loads an individual message by storage ID
func (pg *PostgreSQL) Load(id string) (*data.Message, error) {
	// log.Printf("Get %v", id)
	conn, err := pg.Pool.Acquire(context.TODO())
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	var message data.Message
	query := conn.QueryRow(context.TODO(), "SELECT message FROM messages WHERE message->>'ID' = $1", id)
	switch err := query.Scan(&message); {
	case err == nil:
		return &message, nil
	case err.Error() == "no rows in result set":
		return nil, nil
	default:
		log.Printf("Get error %v", err)
		return nil, err
	}
}
