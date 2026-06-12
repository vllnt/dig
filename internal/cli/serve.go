package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// newServeCmd runs a localhost-only HTTP+JSON daemon over the CLI contract, so
// an in-process SDK or any HTTP client can drive dig without shelling out. It
// binds 127.0.0.1 only — local-first, no remote exposure, no hosted service.
// Handlers run the real dig commands in-process with --json (same adapter
// pattern as `dig mcp`), so there is no logic to drift from the CLI.
func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run a localhost HTTP+JSON daemon over the dig CLI",
		Long: "Exposes the dig surface over HTTP on 127.0.0.1 so apps embed dig without\n" +
			"shelling out: GET /find /recall /drift /log /export (read) and POST /retain\n" +
			"(capture into memory) /org /reconcile /undo (org/reconcile preview unless\n" +
			"?apply=true). Local-first — never binds a public interface.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			host, _, err := net.SplitHostPort(addr)
			if err != nil {
				return fmt.Errorf("bad --addr %q: %w", addr, err)
			}
			if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
				return fmt.Errorf("--addr must be loopback (127.0.0.1 / localhost) — dig serve is local-only")
			}
			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return err
			}
			srv := &http.Server{Handler: digRoutes(), ReadHeaderTimeout: 5 * time.Second}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "dig serving on http://%s — Ctrl-C to stop\n", ln.Addr())
			go func() {
				<-cmd.Context().Done()
				_ = srv.Close()
			}()
			if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "127.0.0.1:3978", "loopback address to bind")
	return cmd
}

// digRoutes builds the daemon's HTTP mux.
func digRoutes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": Version.Version})
	})
	mux.HandleFunc("/find", readHandler(http.MethodGet, func(q reqQuery) []string {
		args := []string{"find", q.get("query"), "--json"}
		if m := q.get("mode"); m != "" {
			args = append(args, "--mode", m)
		}
		if l := q.get("limit"); l != "" {
			args = append(args, "--limit", l)
		}
		return kbArgs(q.get("kb"), args...)
	}))
	mux.HandleFunc("/recall", readHandler(http.MethodGet, func(q reqQuery) []string {
		args := []string{"recall", q.get("query"), "--json"}
		if m := q.get("mode"); m != "" {
			args = append(args, "--mode", m)
		}
		if b := q.get("budget"); b != "" {
			args = append(args, "--budget", b)
		}
		return kbArgs(q.get("kb"), args...)
	}))
	mux.HandleFunc("/retain", retainHTTP)
	mux.HandleFunc("/drift", readHandler(http.MethodGet, func(q reqQuery) []string {
		return kbArgs(q.get("kb"), "drift", "--json")
	}))
	mux.HandleFunc("/log", readHandler(http.MethodGet, func(q reqQuery) []string {
		return kbArgs(q.get("kb"), "log", "--json")
	}))
	mux.HandleFunc("/export", readHandler(http.MethodGet, func(q reqQuery) []string {
		args := []string{"export"}
		if f := q.get("filter"); f != "" {
			args = append(args, "--filter", f)
		}
		if at := q.get("at"); at != "" {
			args = append(args, "--at", at)
		}
		return kbArgs(q.get("kb"), args...)
	}))
	mux.HandleFunc("/org", mutateHTTP("org"))
	mux.HandleFunc("/reconcile", mutateHTTP("reconcile"))
	mux.HandleFunc("/undo", writeHandler(func(q reqQuery) []string {
		return kbArgs(q.get("kb"), "undo")
	}))
	return mux
}

// url adapts query params to a small getter.
type reqQuery struct{ r *http.Request }

func (u reqQuery) get(k string) string { return u.r.URL.Query().Get(k) }

// readHandler serves a GET endpoint that runs a dig --json command.
func readHandler(method string, build func(reqQuery) []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			writeErr(w, http.StatusMethodNotAllowed, "use "+method)
			return
		}
		runDig(w, build(reqQuery{r}))
	}
}

// writeHandler serves a POST endpoint (mutations go through POST).
func writeHandler(build func(reqQuery) []string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "use POST")
			return
		}
		runDig(w, build(reqQuery{r}))
	}
}

// maxRetainBytes caps a single capture over the loopback daemon (32 MiB) — a
// guard against an unbounded body, generous for any real session or document.
const maxRetainBytes = 32 << 20

// retainHTTP captures the POST body into the KB as memory: body = the content,
// ?as= the target path (default a dated memory/ path), ?kb= the KB. It is the
// HTTP form of `dig retain`, so an SDK can write agent memory over the daemon.
func retainHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "use POST")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRetainBytes))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "read body: "+err.Error())
		return
	}
	q := reqQuery{r}
	args := []string{"retain"}
	if as := q.get("as"); as != "" {
		args = append(args, "--as", as)
	}
	runDigStdin(w, string(body), kbArgs(q.get("kb"), args...))
}

// mutateHTTP serves org/reconcile: a preview unless ?apply=true commits.
func mutateHTTP(command string) http.HandlerFunc {
	return writeHandler(func(q reqQuery) []string {
		args := []string{command}
		if apply, _ := strconv.ParseBool(q.get("apply")); !apply {
			args = append(args, "--dry-run")
		}
		return kbArgs(q.get("kb"), args...)
	})
}

// runDig executes the in-process CLI and writes its output. dig's --json
// output is forwarded verbatim with a JSON content type; non-JSON output (plain
// commands) is wrapped as {"output": ...}.
func runDig(w http.ResponseWriter, args []string) {
	out, err := digJSON(args...)
	writeDigResult(w, out, err)
}

// runDigStdin is runDig with a request body fed to the command's stdin (for the
// capture endpoint, whose CLI form reads content from stdin).
func runDigStdin(w http.ResponseWriter, stdin string, args []string) {
	out, err := digJSONStdin(stdin, args...)
	writeDigResult(w, out, err)
}

// writeDigResult forwards a command's output as JSON, or its error as a 400.
func writeDigResult(w http.ResponseWriter, out string, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": out})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if json.Valid([]byte(out)) {
		_, _ = w.Write([]byte(out))
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"output": out})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}
