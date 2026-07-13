package main

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
)

// dbServer owns the single read-write pebble handle and serves HAS/PUT requests
// from the fuzz-worker clients over a Unix socket. pebble is safe for
// concurrent use, so each connection is handled in its own goroutine.
type dbServer struct {
	db       db
	ln       net.Listener
	stored   atomic.Int64 // codes newly written
	received atomic.Int64 // PUT blobs seen (incl. duplicates)

	wg    sync.WaitGroup // tracks live connection handlers
	mu    sync.Mutex     // guards conns / closing
	conns map[net.Conn]struct{}
	closing bool
}

func newServer(db db, ln net.Listener) *dbServer {
	return &dbServer{db: db, ln: ln, conns: make(map[net.Conn]struct{})}
}

// serve accepts connections until the listener is closed.
func (s *dbServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			// Listener closed on shutdown; stop quietly.
			return
		}
		if !s.trackConn(conn) {
			conn.Close() // shutting down; refuse new work
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

// trackConn registers a live connection, returning false if the server is
// already shutting down.
func (s *dbServer) trackConn(conn net.Conn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closing {
		return false
	}
	s.conns[conn] = struct{}{}
	return true
}

func (s *dbServer) untrackConn(conn net.Conn) {
	s.mu.Lock()
	delete(s.conns, conn)
	s.mu.Unlock()
}

// shutdown stops accepting connections, closes any that are still open so their
// handlers unblock, and waits for all in-flight handlers to return. After it
// returns, no handler is touching the database, so the caller can safely close
// it without racing an in-flight write.
func (s *dbServer) shutdown() {
	s.ln.Close()
	s.mu.Lock()
	s.closing = true
	for conn := range s.conns {
		conn.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *dbServer) handle(conn net.Conn) {
	defer conn.Close()
	defer s.untrackConn(conn)
	for {
		op, payload, err := readFrame(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, net.ErrClosed) {
				log.Printf("server: read error: %v", err)
			}
			return
		}
		switch op {
		case opHas:
			resp := respAbsent
			if have, err := s.has(payload); err != nil {
				log.Printf("server: has error: %v", err)
			} else if have {
				resp = respPresent
			}
			if err := writeByte(conn, resp); err != nil {
				return
			}
		case opPut:
			if err := s.put(payload); err != nil {
				log.Printf("server: put error: %v", err)
			}
			if err := writeByte(conn, respAck); err != nil {
				return
			}
		default:
			log.Printf("server: unknown opcode %q", op)
			return
		}
	}
}

// has reports whether a code with the given 32-byte hash is already stored.
func (s *dbServer) has(hash []byte) (bool, error) {
	if _, err := s.db.Get(hash); err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// put stores each blob that isn't already present, atomically per PUT.
//
// The stored counter can slightly over-count under concurrency: two connections
// PUTing the same new blob may both pass the Get check before either writes, so
// both increment stored. Pebble dedups on the key, so this is a stats-only
// imprecision, never data corruption. Serializing put would remove it at the
// cost of throughput on the hot path, which isn't worth it for a progress
// counter.
func (s *dbServer) put(payload []byte) error {
	blobs, err := decodeBlobs(payload)
	if err != nil {
		return err
	}
	var keys, vals [][]byte
	for _, code := range blobs {
		s.received.Add(1)
		key := makeKey(code)
		if _, err := s.db.Get(key); err == nil {
			continue // already stored
		} else if !isNotFound(err) {
			return err
		}
		keys = append(keys, key)
		vals = append(vals, code)
	}
	if len(keys) == 0 {
		return nil
	}
	if err := s.db.SetBatch(keys, vals); err != nil {
		return err
	}
	s.stored.Add(int64(len(keys)))
	return nil
}

func writeByte(w io.Writer, b byte) error {
	_, err := w.Write([]byte{b})
	return err
}
