// Command dmctl operates the reference MDM server's store: it lists
// enrollments, queues commands from a file, and inspects device facts,
// command status and results.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/deploymenttheory/go-microsoft-dm/mdmprotocol/mdm"
	"github.com/deploymenttheory/go-microsoft-dm/paging"
	"github.com/deploymenttheory/go-microsoft-dm/server/internal/app"
	"github.com/deploymenttheory/go-microsoft-dm/server/sqlstore"
	"github.com/deploymenttheory/go-microsoft-dm/storage"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dmctl:", err)
		os.Exit(1)
	}
}

const usage = `usage: dmctl <command> [args]

commands:
  enrollments                 list enrollments
  facts <deviceID>            show a device's reported facts
  commands <deviceID>         list a device's queued commands
  results <deviceID>          show a device's command results
  events <deviceID>           show a device's event log
  queue <deviceID> <file>     queue commands from a file (one per line:
                                replace <uri> <value> [format]
                                add <uri> <value> [format]
                                get <uri>
                                delete <uri>
                                exec <uri> [value])

configuration comes from DM_STORE and DM_DSN (as for dmserver).`

func run(args []string) error {
	if len(args) == 0 {
		fmt.Println(usage)
		return nil
	}
	cfg, err := app.Load()
	if err != nil {
		return err
	}
	dsn := cfg.DSN
	if cfg.Memory {
		return errors.New("dmctl needs a persistent DM_STORE (sqlite, postgres or mysql), not memory")
	}
	ctx := context.Background()
	store, err := sqlstore.Open(ctx, cfg.Store, dsn, sqlstore.Options{})
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	switch args[0] {
	case "enrollments":
		return listEnrollments(ctx, store)
	case "facts":
		return withDevice(args, func(id string) error { return showFacts(ctx, store, id) })
	case "commands":
		return withDevice(args, func(id string) error { return listCommands(ctx, store, id) })
	case "results":
		return withDevice(args, func(id string) error { return showResults(ctx, store, id) })
	case "events":
		return withDevice(args, func(id string) error { return showEvents(ctx, store, id) })
	case "queue":
		if len(args) != 3 {
			return errors.New("usage: dmctl queue <deviceID> <file>")
		}
		return queueFromFile(ctx, store, args[1], args[2])
	default:
		return fmt.Errorf("unknown command %q; run dmctl with no arguments for usage", args[0])
	}
}

func withDevice(args []string, fn func(string) error) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: dmctl %s <deviceID>", args[0])
	}
	return fn(args[1])
}

func listEnrollments(ctx context.Context, store *sqlstore.Store) error {
	res, err := store.List(ctx, storage.EnrollmentQuery{}, paging.Page{Limit: paging.MaxPageSize})
	if err != nil {
		return err
	}
	if len(res.Items) == 0 {
		fmt.Println("no enrollments")
		return nil
	}
	fmt.Printf("%-38s  %-10s  %-8s  %-24s  %s\n", "DEVICE ID", "STATE", "TYPE", "UPN", "ENROLLED")
	for _, e := range res.Items {
		fmt.Printf("%-38s  %-10s  %-8s  %-24s  %s\n", e.DeviceID, e.State, e.EnrollmentType, e.UPN, e.EnrolledAt.Format(time.RFC3339))
	}
	return nil
}

func showFacts(ctx context.Context, store *sqlstore.Store, deviceID string) error {
	f, err := store.Facts(ctx, deviceID)
	if err != nil {
		return err
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	fmt.Println(string(b))
	return nil
}

func listCommands(ctx context.Context, store *sqlstore.Store, deviceID string) error {
	res, err := store.Queue().List(ctx, deviceID, mdm.Query{}, paging.Page{Limit: paging.MaxPageSize})
	if err != nil {
		return err
	}
	if len(res.Items) == 0 {
		fmt.Println("no commands")
		return nil
	}
	fmt.Printf("%-6s  %-20s  %-14s  %-8s  %s\n", "SEQ", "ID", "STATE", "VERB", "ATTEMPTS")
	for _, c := range res.Items {
		fmt.Printf("%-6d  %-20s  %-14s  %-8s  %d\n", c.Seq, c.ID, c.State, c.Body.Name(), c.Attempts)
	}
	return nil
}

func showResults(ctx context.Context, store *sqlstore.Store, deviceID string) error {
	res, err := store.Queue().List(ctx, deviceID, mdm.Query{}, paging.Page{Limit: paging.MaxPageSize})
	if err != nil {
		return err
	}
	for _, c := range res.Items {
		if c.Result == nil {
			continue
		}
		fmt.Printf("%s (%s): status %d\n", c.ID, c.Body.Name(), c.Result.Status)
		for _, it := range c.Result.Items {
			fmt.Printf("    %s = %s\n", it.LocURI(), it.Data.Text())
		}
		for _, ch := range c.Result.Children {
			fmt.Printf("    [%s] status %d\n", ch.Name, ch.Status)
		}
	}
	return nil
}

func showEvents(ctx context.Context, store *sqlstore.Store, deviceID string) error {
	evs, err := store.Events(ctx, deviceID, 0)
	if err != nil {
		return err
	}
	for _, e := range evs {
		fmt.Printf("%s  %-18s  %s\n", e.At.Format(time.RFC3339), e.Kind, string(e.Detail))
	}
	return nil
}

func queueFromFile(ctx context.Context, store *sqlstore.Store, deviceID, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	q := store.Queue()
	sc := bufio.NewScanner(f)
	n := 0
	for line := 1; sc.Scan(); line++ {
		text := strings.TrimSpace(sc.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		cmd, err := parseCommand(text)
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		qc, err := q.Enqueue(ctx, deviceID, cmd, time.Now())
		if err != nil {
			return fmt.Errorf("line %d: %w", line, err)
		}
		n++
		fmt.Printf("queued %s (%s)\n", qc.ID, qc.Body.Name())
	}
	if err := sc.Err(); err != nil {
		return err
	}
	fmt.Printf("%d command(s) queued for %s\n", n, deviceID)
	return nil
}

// parseCommand reads one command line.
func parseCommand(line string) (*mdm.Command, error) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, fmt.Errorf("expected a verb and a URI: %q", line)
	}
	verb, uri := strings.ToLower(fields[0]), fields[1]
	switch verb {
	case "get":
		return mdm.NewGet([]string{uri})
	case "delete":
		return mdm.NewDelete(uri)
	case "exec":
		return mdm.NewExec(uri, strings.Join(fields[2:], " "))
	case "add", "replace":
		if len(fields) < 3 {
			return nil, fmt.Errorf("%s needs a value", verb)
		}
		value := fields[2]
		var opts []mdm.Option
		if len(fields) >= 4 {
			opts = append(opts, mdm.WithFormat(fields[3]))
		}
		if verb == "add" {
			return mdm.NewAdd(uri, value, opts...)
		}
		return mdm.NewReplace(uri, value, opts...)
	}
	return nil, fmt.Errorf("unknown verb %q", verb)
}
