package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS records (
    id              TEXT PRIMARY KEY,
    project         TEXT NOT NULL,
    task            TEXT,
    session_id      TEXT,
    model           TEXT NOT NULL,
    provider        TEXT,
    prompt          TEXT,
    context         TEXT,
    intent          TEXT,
    output          TEXT,
    result          TEXT,
    quality         INTEGER,
    input_tokens    INTEGER NOT NULL DEFAULT 0,
    output_tokens   INTEGER NOT NULL DEFAULT 0,
    cache_creation  INTEGER NOT NULL DEFAULT 0,
    cache_read      INTEGER NOT NULL DEFAULT 0,
    cost_usd        REAL,
    duration_ms     INTEGER,
    created_at      TEXT NOT NULL,
    meta            TEXT
);

CREATE INDEX IF NOT EXISTS idx_records_project ON records(project);
CREATE INDEX IF NOT EXISTS idx_records_model ON records(model);
CREATE INDEX IF NOT EXISTS idx_records_task ON records(project, task);
CREATE INDEX IF NOT EXISTS idx_records_created ON records(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_records_result ON records(result);

CREATE VIRTUAL TABLE IF NOT EXISTS records_fts USING fts5(
    prompt, context, intent, output,
    content=records,
    content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS records_ai AFTER INSERT ON records BEGIN
    INSERT INTO records_fts(rowid, prompt, context, intent, output)
    VALUES (new.rowid, new.prompt, new.context, new.intent, new.output);
END;

CREATE TRIGGER IF NOT EXISTS records_ad AFTER DELETE ON records BEGIN
    INSERT INTO records_fts(records_fts, rowid, prompt, context, intent, output)
    VALUES ('delete', old.rowid, old.prompt, old.context, old.intent, old.output);
END;

CREATE TABLE IF NOT EXISTS pricing (
    model                   TEXT PRIMARY KEY,
    provider                TEXT NOT NULL,
    input_per_mtok          REAL NOT NULL,
    output_per_mtok         REAL NOT NULL,
    cache_create_multiplier REAL DEFAULT 1.25,
    cache_read_multiplier   REAL DEFAULT 0.1,
    updated_at              TEXT NOT NULL
);
`

// Record represents a captured LLM call.
type Record struct {
	ID            string   `json:"id"`
	Project       string   `json:"project"`
	Task          string   `json:"task,omitempty"`
	SessionID     string   `json:"session_id,omitempty"`
	Model         string   `json:"model"`
	Provider      string   `json:"provider,omitempty"`
	Prompt        string   `json:"prompt,omitempty"`
	Context       string   `json:"context,omitempty"`
	Intent        string   `json:"intent,omitempty"`
	Output        string   `json:"output,omitempty"`
	Result        string   `json:"result,omitempty"`
	Quality       *int     `json:"quality,omitempty"`
	InputTokens   int      `json:"input_tokens"`
	OutputTokens  int      `json:"output_tokens"`
	CacheCreation int      `json:"cache_creation,omitempty"`
	CacheRead     int      `json:"cache_read,omitempty"`
	CostUSD       *float64 `json:"cost_usd,omitempty"`
	DurationMs    *int     `json:"duration_ms,omitempty"`
	CreatedAt     string   `json:"created_at"`
	Meta          string   `json:"meta,omitempty"`
}

// Pricing represents a model pricing entry.
type Pricing struct {
	Model                  string  `json:"model"`
	Provider               string  `json:"provider"`
	InputPerMTok           float64 `json:"input_per_mtok"`
	OutputPerMTok          float64 `json:"output_per_mtok"`
	CacheCreateMultiplier  float64 `json:"cache_create_multiplier"`
	CacheReadMultiplier    float64 `json:"cache_read_multiplier"`
	UpdatedAt              string  `json:"updated_at"`
}

// QueryFilter holds filters for querying records.
type QueryFilter struct {
	Project string
	Task    string
	Model   string
	Result  string
	Last    string // e.g. "24h", "7d", "30d"
	Limit   int
	Full    bool
	Search  string // FTS5 search text
}

// Store manages the SQLite database.
type Store struct {
	db *sql.DB
}

// DefaultDBPath returns the default database path.
func DefaultDBPath() string {
	if p := os.Getenv("TOKEN_EVAL_DB"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".token-eval", "eval.db")
}

// New opens or creates a store at the given path.
func New(dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// NewULID generates a new ULID.
func NewULID() string {
	return ulid.Make().String()
}

// InsertRecord inserts a record into the store.
func (s *Store) InsertRecord(r *Record) error {
	if r.ID == "" {
		r.ID = NewULID()
	}
	if r.CreatedAt == "" {
		r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err := s.db.Exec(`INSERT INTO records
		(id, project, task, session_id, model, provider, prompt, context, intent, output,
		 result, quality, input_tokens, output_tokens, cache_creation, cache_read,
		 cost_usd, duration_ms, created_at, meta)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.ID, r.Project, nullStr(r.Task), nullStr(r.SessionID),
		r.Model, nullStr(r.Provider), nullStr(r.Prompt), nullStr(r.Context),
		nullStr(r.Intent), nullStr(r.Output), nullStr(r.Result), r.Quality,
		r.InputTokens, r.OutputTokens, r.CacheCreation, r.CacheRead,
		r.CostUSD, r.DurationMs, r.CreatedAt, nullStr(r.Meta),
	)
	return err
}

// QueryRecords retrieves records matching the filter.
func (s *Store) QueryRecords(f QueryFilter) ([]Record, error) {
	if f.Limit <= 0 {
		f.Limit = 20
	}

	var (
		where []string
		args  []interface{}
		query string
	)

	if f.Search != "" {
		// FTS5 search — join with FTS table
		where = append(where, "r.rowid IN (SELECT rowid FROM records_fts WHERE records_fts MATCH ?)")
		args = append(args, f.Search)
	}

	if f.Project != "" {
		where = append(where, "r.project = ?")
		args = append(args, f.Project)
	}
	if f.Task != "" {
		where = append(where, "r.task = ?")
		args = append(args, f.Task)
	}
	if f.Model != "" {
		where = append(where, "r.model = ?")
		args = append(args, f.Model)
	}
	if f.Result != "" {
		where = append(where, "r.result = ?")
		args = append(args, f.Result)
	}
	if f.Last != "" {
		cutoff := parseLast(f.Last)
		if cutoff != "" {
			where = append(where, "r.created_at >= ?")
			args = append(args, cutoff)
		}
	}

	selectCols := `r.id, r.project, r.task, r.session_id, r.model, r.provider,
		r.intent, r.result, r.quality, r.input_tokens, r.output_tokens,
		r.cache_creation, r.cache_read, r.cost_usd, r.duration_ms, r.created_at, r.meta`
	if f.Full {
		selectCols = `r.id, r.project, r.task, r.session_id, r.model, r.provider,
		r.prompt, r.context, r.intent, r.output, r.result, r.quality,
		r.input_tokens, r.output_tokens, r.cache_creation, r.cache_read,
		r.cost_usd, r.duration_ms, r.created_at, r.meta`
	}

	query = "SELECT " + selectCols + " FROM records r"
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY r.created_at DESC LIMIT ?"
	args = append(args, f.Limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var r Record
		var task, sessionID, provider, intent, result, meta sql.NullString
		var quality sql.NullInt64
		var costUSD sql.NullFloat64
		var durationMs sql.NullInt64

		if f.Full {
			var prompt, context, output sql.NullString
			err = rows.Scan(&r.ID, &r.Project, &task, &sessionID, &r.Model, &provider,
				&prompt, &context, &intent, &output, &result, &quality,
				&r.InputTokens, &r.OutputTokens, &r.CacheCreation, &r.CacheRead,
				&costUSD, &durationMs, &r.CreatedAt, &meta)
			r.Prompt = prompt.String
			r.Context = context.String
			r.Output = output.String
		} else {
			err = rows.Scan(&r.ID, &r.Project, &task, &sessionID, &r.Model, &provider,
				&intent, &result, &quality, &r.InputTokens, &r.OutputTokens,
				&r.CacheCreation, &r.CacheRead, &costUSD, &durationMs, &r.CreatedAt, &meta)
		}
		if err != nil {
			return nil, err
		}
		r.Task = task.String
		r.SessionID = sessionID.String
		r.Provider = provider.String
		r.Intent = intent.String
		r.Result = result.String
		r.Meta = meta.String
		if quality.Valid {
			q := int(quality.Int64)
			r.Quality = &q
		}
		if costUSD.Valid {
			r.CostUSD = &costUSD.Float64
		}
		if durationMs.Valid {
			d := int(durationMs.Int64)
			r.DurationMs = &d
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// UpsertPricing inserts or replaces a pricing entry.
func (s *Store) UpsertPricing(p Pricing) error {
	if p.CacheCreateMultiplier == 0 {
		p.CacheCreateMultiplier = 1.25
	}
	if p.CacheReadMultiplier == 0 {
		p.CacheReadMultiplier = 0.1
	}
	if p.UpdatedAt == "" {
		p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO pricing
		(model, provider, input_per_mtok, output_per_mtok, cache_create_multiplier, cache_read_multiplier, updated_at)
		VALUES (?,?,?,?,?,?,?)`,
		p.Model, p.Provider, p.InputPerMTok, p.OutputPerMTok,
		p.CacheCreateMultiplier, p.CacheReadMultiplier, p.UpdatedAt,
	)
	return err
}

// GetPricing retrieves pricing for a model.
func (s *Store) GetPricing(model string) (*Pricing, error) {
	var p Pricing
	err := s.db.QueryRow(`SELECT model, provider, input_per_mtok, output_per_mtok,
		cache_create_multiplier, cache_read_multiplier, updated_at FROM pricing WHERE model = ?`, model).
		Scan(&p.Model, &p.Provider, &p.InputPerMTok, &p.OutputPerMTok,
			&p.CacheCreateMultiplier, &p.CacheReadMultiplier, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

// ListPricing returns all pricing entries.
func (s *Store) ListPricing() ([]Pricing, error) {
	rows, err := s.db.Query(`SELECT model, provider, input_per_mtok, output_per_mtok,
		cache_create_multiplier, cache_read_multiplier, updated_at FROM pricing ORDER BY provider, model`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prices []Pricing
	for rows.Next() {
		var p Pricing
		if err := rows.Scan(&p.Model, &p.Provider, &p.InputPerMTok, &p.OutputPerMTok,
			&p.CacheCreateMultiplier, &p.CacheReadMultiplier, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prices = append(prices, p)
	}
	return prices, rows.Err()
}

// DeletePricing removes a pricing entry.
func (s *Store) DeletePricing(model string) error {
	_, err := s.db.Exec("DELETE FROM pricing WHERE model = ?", model)
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func parseLast(last string) string {
	now := time.Now().UTC()
	switch {
	case strings.HasSuffix(last, "h"):
		var h int
		fmt.Sscanf(last, "%dh", &h)
		return now.Add(-time.Duration(h) * time.Hour).Format(time.RFC3339)
	case strings.HasSuffix(last, "d"):
		var d int
		fmt.Sscanf(last, "%dd", &d)
		return now.AddDate(0, 0, -d).Format(time.RFC3339)
	}
	return ""
}
