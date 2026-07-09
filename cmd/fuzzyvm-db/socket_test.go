package main

import (
	"net"
	"path/filepath"
	"testing"
)

func TestSocketRoundTrip(t *testing.T) {
	dir := t.TempDir()
	pdb, err := createDB(dir + "/db.pebble")
	if err != nil {
		t.Fatal(err)
	}
	defer pdb.Close()

	sock := filepath.Join(dir, "db.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv := newServer(pdb, ln)
	go srv.serve()

	cli, err := dialSocketDB(sock)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	code := []byte("some-bytecode-blob")

	// Initially absent.
	if have, err := hasCode(cli, code); err != nil || have {
		t.Fatalf("hasCode before put = (%v, %v), want (false, nil)", have, err)
	}
	// Store it.
	if err := putCode(cli, code); err != nil {
		t.Fatalf("putCode: %v", err)
	}
	// Now present.
	if have, err := hasCode(cli, code); err != nil || !have {
		t.Fatalf("hasCode after put = (%v, %v), want (true, nil)", have, err)
	}
	// Batch of two, one new one dup.
	if err := cli.SetBatch(
		[][]byte{makeKey(code), makeKey([]byte("second"))},
		[][]byte{code, []byte("second")},
	); err != nil {
		t.Fatalf("SetBatch: %v", err)
	}
	if srv.stored.Load() != 2 {
		t.Fatalf("stored = %d, want 2 (first blob + second, dup ignored)", srv.stored.Load())
	}
}

// TestServerShutdownPersistsWrites checks that a PUT acknowledged just before
// shutdown is durably stored: shutdown drains in-flight handlers, then the db
// is closed (flushing the WAL), and reopening finds the key.
func TestServerShutdownPersistsWrites(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/db.pebble"
	pdb, err := createDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, "db.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := newServer(pdb, ln)
	go srv.serve()

	cli, err := dialSocketDB(sock)
	if err != nil {
		t.Fatal(err)
	}
	code := []byte("persist-me")
	if err := putCode(cli, code); err != nil { // ack means the server stored it
		t.Fatalf("putCode: %v", err)
	}
	cli.Close()

	// Shut down like generate does: drain handlers, then close the db.
	srv.shutdown()
	if err := pdb.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen and confirm the write survived.
	reopened, err := createDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if have, err := hasCode(reopened, code); err != nil || !have {
		t.Fatalf("after shutdown+reopen hasCode = (%v, %v), want (true, nil)", have, err)
	}
}
