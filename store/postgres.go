package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/davefinster/uqcs-demo/backend/api"
	"github.com/jmoiron/sqlx"

	"go.opencensus.io/trace"

	// Force pg import for sqlx
	_ "github.com/lib/pq"
)

const eventsTable = "events"

// Postgres implements a persistent store of Event/Attachment protos.
type Postgres struct {
	db *sqlx.DB
	sq squirrel.StatementBuilderType
}

// NewPostgres accepts a connectionParameters string which represents the
// backend database to persist events into.
func NewPostgres(connectionParameters string) (*Postgres, error) {
	db, err := sqlx.Connect("postgres", connectionParameters)
	if err != nil {
		return nil, err
	}
	return &Postgres{
		db: db,
		sq: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}, nil
}

type dbEvent struct {
	ID          string         `db:"id"`
	Title       string         `db:"title"`
	Description sql.NullString `db:"description"`
}

type EventFilter struct {
	TitleMatch     *string
	PageNumber     *int32
	ResultsPerPage *int32
}

func (p *Postgres) FetchEvent(ctx context.Context, id string) (*api.Event, error) {
	ctx, span := trace.StartSpan(ctx, "uqcs.backend.postgres.FetchEvent")
	defer span.End()
	sql, args, err := p.sq.Select("*").From("events").Where(squirrel.Eq{
		"id": id,
	}).ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building sql: %s", err)
	}
	event := &dbEvent{}
	err = p.db.GetContext(ctx, event, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error running sql query: %s", err)
	}
	return &api.Event{
		Id:          event.ID,
		Title:       event.Title,
		Description: event.Description.String,
	}, nil
}

func (p *Postgres) FetchEvents(ctx context.Context, filter *EventFilter) ([]*api.Event, error) {
	ctx, span := trace.StartSpan(ctx, "uqcs.backend.postgres.FetchEvents")
	defer span.End()
	query := p.sq.Select("*").From("events")
	if filter != nil {
		if filter.TitleMatch != nil {
			query = query.Where("title LIKE ?", fmt.Sprint("%", *filter.TitleMatch, "%"))
		}
	}
	var events []*dbEvent
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building sql: %s", err)
	}
	err = p.db.SelectContext(ctx, &events, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error running sql query: %s", err)
	}
	protoEvents := make([]*api.Event, len(events))
	for i, event := range events {
		protoEvents[i] = &api.Event{
			Id:          event.ID,
			Title:       event.Title,
			Description: event.Description.String,
		}
	}
	return protoEvents, nil
}

func (p *Postgres) CreateEvent(ctx context.Context, event *api.Event) (*api.Event, error) {
	ctx, span := trace.StartSpan(ctx, "uqcs.backend.postgres.CreateEvent")
	defer span.End()
	sql, args, err := p.sq.Insert(eventsTable).Columns("title", "description").Values(event.GetTitle(), sql.NullString{
		String: event.GetDescription(),
		Valid:  len(event.GetDescription()) > 0,
	}).Suffix("RETURNING \"id\"").ToSql()
	if err != nil {
		return nil, fmt.Errorf("error building sql: %s", err)
	}
	rows, err := p.db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("error running sql query: %s", err)
	}
	var eventID string
	for rows.Next() {
		rows.Scan(&eventID)
	}
	rows.Close()
	return p.FetchEvent(ctx, eventID)
}
