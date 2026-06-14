// Package hashcache caches SHA-256 hashes of executables keyed on (path, size, mtime, inode).
// Only rehashes when the binary changes. Persisted in bbolt so a restart doesn't rehash.
package hashcache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

var bucket = []byte("hashcache")

// Cache is a persistent hash cache backed by bbolt.
type Cache struct {
	db *bolt.DB
}

// Entry is what we store per file.
type Entry struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
	Mtime  int64  `json:"mtime"` // Unix nano
	Inode  uint64 `json:"inode"`
}

func Open(path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &Cache{db: db}, nil
}

func (c *Cache) Close() error { return c.db.Close() }

// Get returns the SHA-256 hex for the given file, using the cache when valid.
// Returns "" if the file can't be read or hashed.
func (c *Cache) Get(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	inode := inodeOf(path)
	key := cacheKey(path)

	var cached Entry
	_ = c.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		data := b.Get([]byte(key))
		if data != nil {
			_ = json.Unmarshal(data, &cached)
		}
		return nil
	})

	if cached.SHA256 != "" &&
		cached.Size == info.Size() &&
		cached.Mtime == info.ModTime().UnixNano() &&
		cached.Inode == inode {
		return cached.SHA256
	}

	hash, err := hashFile(path)
	if err != nil {
		return ""
	}

	entry := Entry{
		SHA256: hash,
		Size:   info.Size(),
		Mtime:  info.ModTime().UnixNano(),
		Inode:  inode,
	}
	data, _ := json.Marshal(entry)
	_ = c.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return nil
		}
		return b.Put([]byte(key), data)
	})

	return hash
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func cacheKey(path string) string {
	return fmt.Sprintf("v1:%s", path)
}

func inodeOf(path string) uint64 {
	return inodeFromStat(path)
}

// uint64FromBytes is a helper for tests.
func uint64FromBytes(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}
