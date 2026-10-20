package journal

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
)

// Journal owns exclusive append access to one durable SQLite event database.
// Loading policy: Open the SQLite database once, validate its schema and integrity, and acquire exclusive append ownership.
// Caching policy: Read events by sequence; the journal stores durable events and idempotency records, not presentation caches.
type Journal struct {
	db       *sql.DB
	lock     *os.File
	Identity string
}

func Open(path string) (*Journal, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("journal already owned: %w", err)
	}
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_synchronous=FULL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		lock.Close()
		return nil, err
	}
	db.SetMaxOpenConns(1)
	j := &Journal{db: db, lock: lock}
	fail := func(err error) (*Journal, error) { j.Close(); return nil, err }
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS events (sequence INTEGER PRIMARY KEY AUTOINCREMENT,
 command_id TEXT NOT NULL UNIQUE, digest TEXT NOT NULL, proposal TEXT NOT NULL,
 payload BLOB NOT NULL, checksum TEXT NOT NULL, feed BLOB);
 CREATE INDEX IF NOT EXISTS events_proposal ON events(proposal, sequence);
 CREATE INDEX IF NOT EXISTS events_feed ON events(sequence) WHERE length(feed)>0;`)
	if err != nil {
		return fail(err)
	}
	var check string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&check); err != nil {
		return fail(err)
	}
	if check != "ok" {
		return fail(fmt.Errorf("journal integrity: %s", check))
	}
	var schema string
	err = db.QueryRow("SELECT value FROM metadata WHERE key='schema'").Scan(&schema)
	if errors.Is(err, sql.ErrNoRows) {
		b := make([]byte, 16)
		if _, err = rand.Read(b); err != nil {
			return fail(err)
		}
		j.Identity = hex.EncodeToString(b)
		_, err = db.Exec("INSERT INTO metadata VALUES ('schema','1'),('identity',?)", j.Identity)
	} else if err == nil && schema != "1" {
		err = fmt.Errorf("unsupported journal schema %q", schema)
	}
	if err != nil {
		return fail(err)
	}
	if err = db.QueryRow("SELECT value FROM metadata WHERE key='identity'").Scan(&j.Identity); err != nil {
		return fail(err)
	}
	return j, nil
}

func (j *Journal) Close() error {
	err := j.db.Close()
	unlock := syscall.Flock(int(j.lock.Fd()), syscall.LOCK_UN)
	closeErr := j.lock.Close()
	return errors.Join(err, unlock, closeErr)
}

func Digest(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func checksum(data, feed []byte) string {
	hash := sha256.New()
	hash.Write(data)
	hash.Write([]byte{0})
	hash.Write(feed)
	return hex.EncodeToString(hash.Sum(nil))
}

func (j *Journal) Append(r Record) (Record, error) {
	if r.ID == "" || !json.Valid(r.Payload) || (len(r.Feed) > 0 && !json.Valid(r.Feed)) {
		return r, errors.New("invalid journal record")
	}
	tx, err := j.db.Begin()
	if err != nil {
		return r, err
	}
	defer tx.Rollback()
	result, err := tx.Exec("INSERT INTO events(command_id,digest,proposal,payload,checksum,feed) VALUES(?,?,?,?,?,?)", r.ID, r.Digest, r.Proposal, []byte(r.Payload), checksum(r.Payload, r.Feed), []byte(r.Feed))
	if err != nil {
		return r, err
	}
	n, err := result.LastInsertId()
	if err != nil {
		return r, err
	}
	if err = tx.Commit(); err != nil {
		return r, err
	}
	r.Sequence = uint64(n)
	return r, nil
}

func (j *Journal) Lookup(id string) (Record, bool, error) {
	var r Record
	var hash string
	err := j.db.QueryRow("SELECT sequence,command_id,digest,proposal,payload,checksum,COALESCE(feed,x'') FROM events WHERE command_id=?", id).Scan(&r.Sequence, &r.ID, &r.Digest, &r.Proposal, &r.Payload, &hash, &r.Feed)
	if errors.Is(err, sql.ErrNoRows) {
		return r, false, nil
	}
	if err != nil {
		return r, false, err
	}
	if checksum(r.Payload, r.Feed) != hash {
		return r, false, errors.New("journal checksum mismatch")
	}
	return r, true, nil
}

func (j *Journal) Read(after uint64, limit int) ([]Record, error) {
	rows, err := j.db.Query("SELECT sequence,command_id,digest,proposal,payload,checksum,COALESCE(feed,x'') FROM events WHERE sequence>? ORDER BY sequence LIMIT ?", after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Record{}
	for rows.Next() {
		var r Record
		var hash string
		if err = rows.Scan(&r.Sequence, &r.ID, &r.Digest, &r.Proposal, &r.Payload, &hash, &r.Feed); err != nil {
			return nil, err
		}
		if checksum(r.Payload, r.Feed) != hash {
			return nil, fmt.Errorf("journal checksum mismatch at %d", r.Sequence)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// ReadFeed uses the feed index, so subscriptions never read candidate payloads.
func (j *Journal) ReadFeed(after uint64, limit int) ([]Record, error) {
	rows, err := j.db.Query("SELECT sequence,payload,checksum,feed FROM events WHERE sequence>? AND length(feed)>0 ORDER BY sequence LIMIT ?", after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := []Record{}
	for rows.Next() {
		var r Record
		var hash string
		if err = rows.Scan(&r.Sequence, &r.Payload, &hash, &r.Feed); err != nil {
			return nil, err
		}
		if checksum(r.Payload, r.Feed) != hash {
			return nil, fmt.Errorf("feed checksum mismatch at %d", r.Sequence)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}
