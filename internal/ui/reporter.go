package ui

import (
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
)

// Reporter is the sink for all user-facing feedback. Long operations (notably
// hydration) drive Progress so the user sees steady movement rather than a
// silent pause. Implementations must be safe to call with an empty label.
type Reporter interface {
	// Step announces the start of a discrete operation.
	Step(msg string)
	// Progress reports incremental progress toward total (bytes or file count).
	// A total of 0 means indeterminate.
	Progress(label string, done, total int64)
	// Info prints secondary detail.
	Info(msg string)
	// Warn reports a recoverable problem.
	Warn(msg string)
	// Success reports successful completion.
	Success(msg string)
}

// Options configure reporter construction.
type Options struct {
	// Quiet suppresses steps/info/progress, leaving only warnings.
	Quiet bool
	// ForcePlain disables styling and the TTY renderer regardless of isatty.
	ForcePlain bool
}

// New returns the appropriate Reporter for w: a styled TTY reporter when w is a
// terminal and plain output is not forced, otherwise a plain-line reporter.
func New(w io.Writer, opts Options) Reporter {
	if !opts.ForcePlain && isTerminal(w) {
		return &ttyReporter{w: w, theme: NeonTheme(), quiet: opts.Quiet}
	}
	return &plainReporter{w: w, quiet: opts.Quiet}
}

// NewStderr is a convenience for the common case of reporting to stderr (so
// stdout stays clean for machine-readable output).
func NewStderr(opts Options) Reporter {
	return New(os.Stderr, opts)
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
}

// ttyReporter renders styled, single-line-rewriting output for terminals.
type ttyReporter struct {
	w          io.Writer
	theme      Theme
	quiet      bool
	inProgress bool
}

func (r *ttyReporter) endProgress() {
	if r.inProgress {
		fmt.Fprintln(r.w)
		r.inProgress = false
	}
}

func (r *ttyReporter) Step(msg string) {
	if r.quiet {
		return
	}
	r.endProgress()
	fmt.Fprintf(r.w, "%s %s\n", r.theme.Accent.Render("▸"), r.theme.Step.Render(msg))
}

func (r *ttyReporter) Progress(label string, done, total int64) {
	if r.quiet {
		return
	}
	r.inProgress = true
	var body string
	if total > 0 {
		pct := float64(done) / float64(total) * 100
		body = fmt.Sprintf("%s %s", label, r.theme.Muted.Render(fmt.Sprintf("%5.1f%%", pct)))
	} else {
		body = fmt.Sprintf("%s %s", label, r.theme.Muted.Render(fmt.Sprintf("%d", done)))
	}
	// Carriage return rewrites the same line in place.
	fmt.Fprintf(r.w, "\r  %s", body)
}

func (r *ttyReporter) Info(msg string) {
	if r.quiet {
		return
	}
	r.endProgress()
	fmt.Fprintf(r.w, "  %s\n", r.theme.Muted.Render(msg))
}

func (r *ttyReporter) Warn(msg string) {
	r.endProgress()
	fmt.Fprintf(r.w, "%s %s\n", r.theme.Warn.Render("!"), r.theme.Warn.Render(msg))
}

func (r *ttyReporter) Success(msg string) {
	if r.quiet {
		return
	}
	r.endProgress()
	fmt.Fprintf(r.w, "%s %s\n", r.theme.Success.Render("✓"), r.theme.Success.Render(msg))
}

// plainReporter emits unstyled lines suitable for pipes, logs, and --quiet.
type plainReporter struct {
	w     io.Writer
	quiet bool
}

func (r *plainReporter) Step(msg string) {
	if r.quiet {
		return
	}
	fmt.Fprintf(r.w, "> %s\n", msg)
}

func (r *plainReporter) Progress(label string, done, total int64) {
	// Plain output does not rewrite lines; progress is intentionally silent to
	// avoid flooding logs. Completion is reported via Step/Success.
}

func (r *plainReporter) Info(msg string) {
	if r.quiet {
		return
	}
	fmt.Fprintf(r.w, "  %s\n", msg)
}

func (r *plainReporter) Warn(msg string) {
	fmt.Fprintf(r.w, "warning: %s\n", msg)
}

func (r *plainReporter) Success(msg string) {
	if r.quiet {
		return
	}
	fmt.Fprintf(r.w, "%s\n", msg)
}
