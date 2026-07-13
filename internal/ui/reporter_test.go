package ui

import (
	"bytes"
	"strings"
	"testing"
)

// A bytes.Buffer is not a *os.File, so New must select the plain reporter.
func TestNew_NonTerminalSelectsPlain(t *testing.T) {
	r := New(&bytes.Buffer{}, Options{})
	if _, ok := r.(*plainReporter); !ok {
		t.Fatalf("expected plainReporter for non-terminal writer, got %T", r)
	}
}

func TestPlainReporter_Output(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, Options{})
	r.Step("hydrating workspace")
	r.Info("42 files")
	r.Success("done")

	out := buf.String()
	for _, want := range []string{"hydrating workspace", "42 files", "done"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q; got:\n%s", want, out)
		}
	}
	// Plain output must carry no ANSI escape codes.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("plain reporter emitted ANSI escapes:\n%q", out)
	}
}

func TestPlainReporter_QuietSuppressesSteps(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, Options{Quiet: true})
	r.Step("noisy")
	r.Info("noisy")
	r.Warn("important")

	out := buf.String()
	if strings.Contains(out, "noisy") {
		t.Errorf("quiet mode should suppress steps/info; got:\n%s", out)
	}
	if !strings.Contains(out, "important") {
		t.Errorf("quiet mode must still surface warnings; got:\n%s", out)
	}
}
