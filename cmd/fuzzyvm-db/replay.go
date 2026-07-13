package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
)

// Env vars used to pass parameters into TestReplayCorpus. They live here (a
// non-test file) rather than in replay_test.go so both the command and the test
// can reference them — a test file is not part of the non-test build.
const (
	replayDBEnv      = "FUZZYVM_REPLAY_DB"
	replayLimitEnv   = "FUZZYVM_REPLAY_LIMIT"
	replayWorkersEnv = "FUZZYVM_REPLAY_WORKERS"
)

// defaultCoverPkg scopes coverage instrumentation to go-ethereum's EVM. The
// report answers "which parts of core/vm does the corpus reach".
const defaultCoverPkg = "github.com/ethereum/go-ethereum/core/vm/..."

var replayCommand = &cli.Command{
	Name:   "replay",
	Usage:  "replay the stored corpus through the EVM under coverage instrumentation and report which core/vm paths are reached",
	Action: replay,
	Flags: []cli.Flag{
		dbFlag,
		&cli.StringFlag{
			Name:  "coverpkg",
			Usage: "package pattern to instrument for coverage",
			Value: defaultCoverPkg,
		},
		&cli.StringFlag{
			Name:  "coverprofile",
			Usage: "path to write the coverage profile",
			Value: "cover.out",
		},
		&cli.StringFlag{
			Name:  "html",
			Usage: "if set, render the coverage profile to this HTML file",
		},
		&cli.IntFlag{
			Name:  "limit",
			Usage: "replay at most this many codes (0 = all)",
			Value: 0,
		},
		&cli.IntFlag{
			Name:    "workers",
			Aliases: []string{"w"},
			Usage:   "number of parallel replay workers (0 = one per CPU)",
			Value:   0,
		},
		&cli.DurationFlag{
			Name:  "timeout",
			Usage: "test timeout; large corpora may need hours",
			Value: 0,
		},
	},
}

// replay measures which parts of go-ethereum's core/vm the stored corpus
// reaches. Go coverage is compile-time instrumentation, and the already-built
// fuzzyvm-db binary carries none for its dependencies, so the only way to
// instrument core/vm is `go test -coverpkg=...`. This command therefore shells
// out to `go test` (mirroring how `generate` shells out to `go test -fuzz`),
// pointing it at TestReplayCorpus, then summarises the resulting profile.
func replay(ctx *cli.Context) error {
	dbPath, err := filepath.Abs(ctx.String(dbFlag.Name))
	if err != nil {
		return err
	}
	// The profile/HTML paths are written relative to our cwd, while `go test`
	// runs in the package dir — resolve them absolute so output lands
	// predictably.
	profile, err := filepath.Abs(ctx.String("coverprofile"))
	if err != nil {
		return err
	}
	pkgDir, err := packageDir()
	if err != nil {
		return err
	}

	args := []string{
		"test",
		"-run=^TestReplayCorpus$",
		"-coverpkg=" + ctx.String("coverpkg"),
		"-coverprofile=" + profile,
		// Workers execute concurrently, so coverage counters race under the
		// default set/count modes.
		"-covermode=atomic",
		"-v",
	}
	if d := ctx.Duration("timeout"); d > 0 {
		args = append(args, fmt.Sprintf("-timeout=%s", d))
	} else {
		args = append(args, "-timeout=2h")
	}
	args = append(args, pkgDir)

	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	env := append(os.Environ(), fmt.Sprintf("%s=%s", replayDBEnv, dbPath))
	if l := ctx.Int("limit"); l > 0 {
		env = append(env, fmt.Sprintf("%s=%d", replayLimitEnv, l))
	}
	if w := ctx.Int("workers"); w > 0 {
		env = append(env, fmt.Sprintf("%s=%d", replayWorkersEnv, w))
	}
	cmd.Env = env

	fmt.Printf("Replaying corpus %v (coverpkg=%s)\n", dbPath, ctx.String("coverpkg"))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("replay test failed: %w", err)
	}

	if err := summarize(profile); err != nil {
		return err
	}

	if html := ctx.String("html"); html != "" {
		htmlAbs, err := filepath.Abs(html)
		if err != nil {
			return err
		}
		out := exec.Command("go", "tool", "cover", "-html="+profile, "-o="+htmlAbs)
		out.Stderr = os.Stderr
		if err := out.Run(); err != nil {
			return fmt.Errorf("rendering HTML report: %w", err)
		}
		fmt.Printf("HTML report written to %v\n", htmlAbs)
	}
	return nil
}

// summarize parses `go tool cover -func` and prints the total coverage plus the
// never-entered (0.0%) functions grouped by file, so the output reads as "these
// areas are untouched" rather than a flat list.
func summarize(profile string) error {
	out, err := exec.Command("go", "tool", "cover", "-func="+profile).Output()
	if err != nil {
		return fmt.Errorf("summarising coverage: %w", err)
	}
	// Each line is: <file>:<line>:\t<func>\t<pct>%. The final line is total:.
	uncovered := map[string][]string{}
	var files []string
	total := ""
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		loc, fn, pct := fields[0], fields[1], fields[len(fields)-1]
		if strings.HasPrefix(loc, "total:") {
			total = pct
			continue
		}
		p, perr := strconv.ParseFloat(strings.TrimSuffix(pct, "%"), 64)
		if perr != nil || p != 0.0 {
			continue
		}
		file := loc
		if i := strings.IndexByte(loc, ':'); i >= 0 {
			file = loc[:i]
		}
		if _, seen := uncovered[file]; !seen {
			files = append(files, file)
		}
		uncovered[file] = append(uncovered[file], fn)
	}
	if err := sc.Err(); err != nil {
		return err
	}

	fmt.Printf("\nTotal core/vm statement coverage: %s\n", total)
	if len(files) == 0 {
		fmt.Println("No fully-uncovered functions.")
		return nil
	}
	// Sort files by number of uncovered functions, descending.
	sort.Slice(files, func(i, j int) bool {
		if len(uncovered[files[i]]) != len(uncovered[files[j]]) {
			return len(uncovered[files[i]]) > len(uncovered[files[j]])
		}
		return files[i] < files[j]
	})
	fmt.Printf("\nNever-entered functions (grouped by file):\n")
	for _, file := range files {
		fns := uncovered[file]
		sort.Strings(fns)
		fmt.Printf("\n  %s (%d):\n", file, len(fns))
		for _, fn := range fns {
			fmt.Printf("    %s\n", fn)
		}
	}
	return nil
}
