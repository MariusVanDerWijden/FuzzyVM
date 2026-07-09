package main

import (
	"fmt"
	"net"
	"sync"

	"github.com/cockroachdb/pebble"
)

// socketDB is a db implementation backed by the generate server over a Unix
// socket. It lets the fuzz worker reuse run()/hasCode() unchanged: Get becomes a
// HAS query (returning pebble.ErrNotFound when absent, so the pre-minimize skip
// still works) and Set/SetBatch become PUT messages to the DB-owning server.
type socketDB struct {
	mu   sync.Mutex // one request/response at a time per connection
	conn net.Conn
}

func dialSocketDB(addr string) (*socketDB, error) {
	conn, err := net.Dial("unix", addr)
	if err != nil {
		return nil, err
	}
	return &socketDB{conn: conn}, nil
}

// Get returns (non-nil, nil) if the code hash is present and
// (nil, pebble.ErrNotFound) if not, mirroring pebbleDB.Get closely enough for
// hasCode. The actual value is never needed by callers, so we don't ship it.
func (s *socketDB) Get(key []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeFrame(s.conn, opHas, key); err != nil {
		return nil, err
	}
	resp, err := readByte(s.conn)
	if err != nil {
		return nil, err
	}
	switch resp {
	case respPresent:
		return []byte{}, nil
	case respAbsent:
		return nil, pebble.ErrNotFound
	default:
		return nil, fmt.Errorf("unexpected HAS response %q", resp)
	}
}

func (s *socketDB) Set(key, value []byte) error {
	// The server keys by hash of the code, so only the code (value) is shipped.
	return s.putBlobs(value)
}

func (s *socketDB) SetBatch(keys, values [][]byte) error {
	return s.putBlobs(values...)
}

func (s *socketDB) putBlobs(blobs ...[]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := writeFrame(s.conn, opPut, encodeBlobs(blobs...)); err != nil {
		return err
	}
	resp, err := readByte(s.conn)
	if err != nil {
		return err
	}
	if resp != respAck {
		return fmt.Errorf("unexpected PUT response %q", resp)
	}
	return nil
}

func (s *socketDB) Close() error {
	return s.conn.Close()
}
