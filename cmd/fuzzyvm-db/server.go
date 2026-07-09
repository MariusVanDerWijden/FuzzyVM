package main

import (
	"errors"
	"io"
	"log"
	"net"
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
}

func newServer(db db, ln net.Listener) *dbServer {
	return &dbServer{db: db, ln: ln}
}

// serve accepts connections until the listener is closed.
func (s *dbServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			// Listener closed on shutdown; stop quietly.
			return
		}
		go s.handle(conn)
	}
}

func (s *dbServer) handle(conn net.Conn) {
	defer conn.Close()
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
