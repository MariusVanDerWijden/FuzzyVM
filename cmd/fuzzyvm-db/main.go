package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"syscall"

	"github.com/MariusVanDerWijden/FuzzyVM/filler"
	"github.com/MariusVanDerWijden/FuzzyVM/fuzzer"
	"github.com/MariusVanDerWijden/FuzzyVM/generator"
	"github.com/cockroachdb/pebble"
	"github.com/urfave/cli/v2"
)

const (
	// defaultDBFile is used when -db is not supplied.
	defaultDBFile = "fuzzyvm-db.pebble"
	// sockEnvKey names the env var carrying the Unix socket path that the
	// FuzzEVM worker subprocesses (spawned by `generate` via `go test -fuzz`)
	// use to reach the database-owning server. pebble locks the directory
	// exclusively, so workers can't open their own handle; they talk to the one
	// writer over this socket instead.
	sockEnvKey = "FUZZYVM_SOCK"
	// debugEnvKey, when set to "1", makes the FuzzEVM worker subprocesses log
	// every generation strategy they select. `generate` propagates the --debug
	// flag to the workers through it (the workers, not the parent, do the
	// generating).
	debugEnvKey = "FUZZYVM_DEBUG"
)

// debugFlag enables logging of the chosen generation strategies to the console.
var debugFlag = &cli.BoolFlag{
	Name:  "debug",
	Usage: "log the generation strategies chosen for each program to the console",
}

var dbFlag = &cli.StringFlag{
	Name:  "db",
	Usage: "path to the pebble database",
	Value: defaultDBFile,
}

var inspectCommand = &cli.Command{
	Name:   "inspect",
	Usage:  "print statistics about an existing database",
	Action: inspect,
	Flags:  []cli.Flag{dbFlag},
}

var generateCommand = &cli.Command{
	Name:   "generate",
	Usage:  "coverage-guided fuzzing that fills the database with EVM bytecodes",
	Action: generate,
	Flags: []cli.Flag{
		dbFlag,
		&cli.IntFlag{
			Name:    "procs",
			Aliases: []string{"p"},
			Usage:   "number of parallel fuzzing workers (0 = one per CPU)",
			Value:   0,
		},
		&cli.DurationFlag{
			Name:  "time",
			Usage: "how long to fuzz for (0 = until interrupted)",
			Value: 0,
		},
		debugFlag,
	},
}

func main() {
	app := cli.NewApp()
	app.Name = "fuzzyvm-db"
	app.Usage = "build and inspect a content-addressed database of EVM bytecodes"
	app.Commands = []*cli.Command{
		inspectCommand,
		generateCommand,
		replayCommand,
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// socketAddr returns the server socket path for the FuzzEVM harness, or "" if
// unset (in which case the harness falls back to a local pebble handle, e.g.
// when run directly via `go test`).
func socketAddr() string {
	return os.Getenv(sockEnvKey)
}

// inspect prints statistics about an existing database.
func inspect(ctx *cli.Context) error {
	path := ctx.String(dbFlag.Name)
	// Open read-only so the stats command neither creates an empty db
	// nor contends with a running fuzzer for the directory lock.
	db, err := pebble.Open(path, &pebble.Options{ReadOnly: true, ErrorIfNotExists: true})
	if err != nil {
		return err
	}
	defer db.Close()

	metrics := db.Metrics()
	fmt.Printf("Reading metrics for %v\n", path)
	fmt.Printf("Estimated disk usage: %.2fM\n", float64(metrics.DiskSpaceUsage())/1024/1024)
	fmt.Printf("Key count: %v\n", countKeys(db))
	return nil
}

// generate runs coverage-guided fuzzing that fills the database with EVM
// bytecodes, scaling across CPU cores.
//
// pebble locks its directory exclusively, so only this process can hold the
// database open. It therefore opens pebble read-write, listens on a Unix
// socket, and spawns `go test -fuzz=FuzzEVM -parallel=N`. Each worker
// subprocess gets independent coverage guidance from the Go fuzzer, checks the
// database with a HAS query over the socket before the expensive minimization
// step, and streams new bytecodes back with PUT for this process to store.
func generate(ctx *cli.Context) error {
	procs := ctx.Int("procs")
	if procs <= 0 {
		procs = runtime.NumCPU()
	}
	dbPath, err := filepath.Abs(ctx.String(dbFlag.Name))
	if err != nil {
		return err
	}
	pkgDir, err := packageDir()
	if err != nil {
		return err
	}

	// Open the one read-write handle to the database.
	pdb, err := createDB(dbPath)
	if err != nil {
		return err
	}

	// Listen on a Unix socket in a temp dir; the path goes to the workers.
	sockDir, err := os.MkdirTemp("", "fuzzyvm-sock-")
	if err != nil {
		pdb.Close()
		return err
	}
	defer os.RemoveAll(sockDir)
	sockPath := filepath.Join(sockDir, "db.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		pdb.Close()
		return err
	}

	srv := newServer(pdb, ln)
	go srv.serve()

	args := []string{
		"test",
		"-run=^$",         // don't run unit tests, only fuzz
		"-fuzz=^FuzzEVM$", // the harness that fills the db
		fmt.Sprintf("-parallel=%d", procs),
		// Raise the per-input hang deadline: some generated programs (heavy
		// BLS/KZG precompile calls, then minimization) legitimately take a while,
		// and the default would flag them as hangs.
		"-timeout=30s",
	}
	if d := ctx.Duration("time"); d > 0 {
		args = append(args, fmt.Sprintf("-fuzztime=%s", d))
	}
	args = append(args, pkgDir)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", sockEnvKey, sockPath))
	if ctx.Bool(debugFlag.Name) {
		// The workers, not this process, do the generating, so pass the flag
		// down. With multiple parallel workers the strategy logs will interleave.
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=1", debugEnvKey))
	}

	// Ctrl-C is the normal way to stop an open-ended run. SIGINT reaches both
	// this process and the `go test` child (same process group); we catch it
	// here so the child exits cleanly and control returns to the shutdown
	// sequence below, which flushes the database. Without this, the default
	// SIGINT action would kill us before pdb.Close() runs and every unsynced
	// (NoSync) write would be lost.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	var interrupted atomic.Bool
	go func() {
		if _, ok := <-sigCh; ok {
			interrupted.Store(true)
		}
	}()

	fmt.Printf("Fuzzing into %v with %d workers\n", dbPath, procs)
	err = cmd.Run()

	// Drain in-flight handlers, then flush and close the database. shutdown()
	// closes the listener and waits for every connection handler to finish, so
	// no PUT is still writing when we Close the db.
	srv.shutdown()
	if cerr := pdb.Close(); cerr != nil && err == nil {
		err = cerr
	}
	fmt.Printf("Stored %d new codes (%d candidates received)\n", srv.stored.Load(), srv.received.Load())
	if interrupted.Load() {
		// A user-initiated Ctrl-C is a clean stop, not a failure. (A genuine
		// fuzz-found crash exits without a signal, so its error still surfaces.)
		return nil
	}
	return err
}

// packageDir returns the directory of this command's Go package, so `go test`
// can be pointed at it from any working directory.
func packageDir() (string, error) {
	out, err := exec.Command("go", "list", "-f", "{{.Dir}}", "github.com/MariusVanDerWijden/FuzzyVM/cmd/fuzzyvm-db").Output()
	if err != nil {
		return "", fmt.Errorf("locating fuzzyvm-db package (is the Go toolchain available?): %w", err)
	}
	return string(bytes.TrimSpace(out)), nil
}

func countKeys(db *pebble.DB) int {
	iter, err := db.NewIter(&pebble.IterOptions{})
	if err != nil {
		panic(err)
	}
	defer iter.Close()
	keys := 0
	for iter.First(); iter.Valid(); iter.Next() {
		keys++
	}
	return keys
}

func createDB(file string) (*pebbleDB, error) {
	pdb, err := pebble.Open(file, &pebble.Options{})
	if err != nil {
		return nil, err
	}
	return &pebbleDB{db: pdb}, nil
}

func makeKey(code []byte) []byte {
	hash := sha256.Sum256(code)
	return hash[:]
}

type db interface {
	Get(key []byte) ([]byte, error)
	Set(key, value []byte) error
	SetBatch(keys, values [][]byte) error
	Close() error
}

type pebbleDB struct {
	db *pebble.DB
}

func (db *pebbleDB) Get(key []byte) ([]byte, error) {
	val, closer, err := db.db.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	// The slice returned by pebble is only valid until closer is closed.
	return bytes.Clone(val), nil
}

func (db *pebbleDB) Set(key, value []byte) error {
	return db.db.Set(key, value, pebble.NoSync)
}

func (db *pebbleDB) SetBatch(keys, values [][]byte) error {
	batch := db.db.NewBatch()
	defer batch.Close()
	for i, key := range keys {
		if err := batch.Set(key, values[i], nil); err != nil {
			return err
		}
	}
	return batch.Commit(pebble.NoSync)
}

func (db *pebbleDB) Close() error {
	return db.db.Close()
}

// isNotFound reports whether err signals a missing key, from either a direct
// pebble handle or the socketDB (which returns pebble.ErrNotFound too).
func isNotFound(err error) bool {
	return errors.Is(err, pebble.ErrNotFound)
}

func hasCode(db db, code []byte) (bool, error) {
	if _, err := db.Get(makeKey(code)); isNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

func putCode(db db, code []byte) error {
	return db.Set(makeKey(code), code)
}

func run(db db, input []byte) error {
	// Too little data destroys our performance and makes it hard for the generator
	if len(input) < 32 {
		return nil
	}
	f := filler.NewFiller(input)
	gst, bytecode := generator.GenerateProgram(f)
	if have, err := hasCode(db, bytecode); err != nil {
		return err
	} else if have {
		// already have this code in our db, skip
		return nil
	}
	_, minCode, err := fuzzer.MinimizeProgram(gst)
	if errors.Is(err, fuzzer.ErrTraceTooLarge) {
		// The trace is too large to run, so there's no way to minimize it.
		// Still worth keeping: store the full bytecode as-is.
		return putCode(db, bytecode)
	} else if err != nil {
		// A program that fails to minimize is not worth stopping a campaign for.
		log.Printf("skipping program that failed to minimize: %v", err)
		return nil
	}
	if have, err := hasCode(db, minCode); err != nil {
		return err
	} else if have {
		// a different program already minimized to this code
		return putCode(db, bytecode)
	}
	// Store both codes atomically so a failure between the writes can't leave
	// the full code present (and thus skipped forever) without its minimized
	// counterpart.
	return db.SetBatch(
		[][]byte{makeKey(bytecode), makeKey(minCode)},
		[][]byte{bytecode, minCode},
	)
}
