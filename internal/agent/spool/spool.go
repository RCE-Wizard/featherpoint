// Package spool is a size-capped, bbolt-backed store-and-forward queue for outbound batches.
// Batches survive server outages and are replayed on reconnect.
// Because ingest is idempotent on batch_id, replays are safe.
package spool

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

var spoolBucket = []byte("spool")

// Spool is a persistent FIFO queue of JSON-encoded outbound batches.
type Spool struct {
	db       *bolt.DB
	maxBytes int64
}

func Open(dir string, maxBytes int64) (*Spool, error) {
	db, err := bolt.Open(filepath.Join(dir, "spool.db"), 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(spoolBucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Spool{db: db, maxBytes: maxBytes}, nil
}

func (s *Spool) Close() error { return s.db.Close() }

// Push adds an item to the spool. Drops the oldest entry if over the size cap.
func (s *Spool) Push(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(spoolBucket)

		// Enforce size cap
		for int64(b.Stats().LeafInuse) > s.maxBytes && b.Stats().KeyN > 0 {
			c := b.Cursor()
			k, _ := c.First()
			if k != nil {
				_ = b.Delete(k)
			} else {
				break
			}
		}

		seq, _ := b.NextSequence()
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, seq)
		return b.Put(key, data)
	})
}

// Peek returns the oldest item without removing it.
func (s *Spool) Peek(v interface{}) (key []byte, err error) {
	err = s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(spoolBucket)
		c := b.Cursor()
		k, val := c.First()
		if k == nil {
			return fmt.Errorf("empty")
		}
		key = make([]byte, len(k))
		copy(key, k)
		return json.Unmarshal(val, v)
	})
	return key, err
}

// Ack removes the item with the given key (after successful delivery).
func (s *Spool) Ack(key []byte) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(spoolBucket).Delete(key)
	})
}

// Len returns the number of items in the spool.
func (s *Spool) Len() int {
	var n int
	_ = s.db.View(func(tx *bolt.Tx) error {
		n = tx.Bucket(spoolBucket).Stats().KeyN
		return nil
	})
	return n
}
