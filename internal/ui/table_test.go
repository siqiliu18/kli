package ui

import (
	"strings"
	"testing"

	kt "github.com/siqiliu/kli/internal/types"
)

func TestColorLogLine(t *testing.T) {
	tests := []struct {
		line     string
		wantText string // original text must survive (color codes wrap it, not replace it)
	}{
		{"[ERROR] something broke", "[ERROR] something broke"},
		{"level=error msg=oops", "level=error msg=oops"},
		{"FATAL: unrecoverable", "FATAL: unrecoverable"},
		{"[WARN] disk usage high", "[WARN] disk usage high"},
		{"level=warn msg=slow query", "level=warn msg=slow query"},
		{"INFO: server started on :8080", "INFO: server started on :8080"},
		{"plain log line no level", "plain log line no level"},
	}

	for _, tc := range tests {
		got := ColorLogLine(tc.line)
		if !strings.Contains(got, tc.wantText) {
			t.Errorf("ColorLogLine(%q) = %q, want it to contain %q", tc.line, got, tc.wantText)
		}
	}
}

func TestColorLogLine_ErrorVariants(t *testing.T) {
	// ERROR and FATAL should both produce a non-plain result (lipgloss wraps with ANSI codes)
	// We can't compare exact ANSI output, but ERROR lines must differ from INFO lines
	errLine := ColorLogLine("[ERROR] broke")
	infoLine := ColorLogLine("[INFO] fine")
	if errLine == infoLine {
		t.Error("ERROR and INFO lines should produce different colored output")
	}

	fatalLine := ColorLogLine("FATAL: crash")
	if fatalLine == infoLine {
		t.Error("FATAL and INFO lines should produce different colored output")
	}

	warnLine := ColorLogLine("[WARN] slow")
	if warnLine == infoLine {
		t.Error("WARN and INFO lines should produce different colored output")
	}
}

func TestPodPhaseDisplay_Hints(t *testing.T) {
	// These phases should produce a "kubectl describe" hint
	hintPhases := []string{
		"ContainerCreating",
		"PodInitializing",
		"ImagePullBackOff",
		"ErrImagePull",
		"InvalidImageName",
		"CreateContainerConfigError",
		"RunContainerError",
		"OOMKilled",
		"Error",
	}
	for _, phase := range hintPhases {
		_, hint := podPhaseDisplay("my-pod", phase)
		if hint == "" {
			t.Errorf("podPhaseDisplay(_, %q) should return a hint", phase)
		}
		if !strings.Contains(hint, "my-pod") {
			t.Errorf("podPhaseDisplay(_, %q) hint %q should contain pod name", phase, hint)
		}
	}
}

func TestPodPhaseDisplay_NoHint(t *testing.T) {
	// These phases should NOT produce a hint
	noHintPhases := []string{"Running", "Pending", "Succeeded", "CrashLoopBackOff", "Terminating"}
	for _, phase := range noHintPhases {
		_, hint := podPhaseDisplay("my-pod", phase)
		if hint != "" {
			t.Errorf("podPhaseDisplay(_, %q) hint = %q, want empty", phase, hint)
		}
	}
}

func TestHealthDisplay(t *testing.T) {
	tests := []struct {
		state     kt.HealthState
		wantLabel string
	}{
		{kt.Healthy, "Healthy"},
		{kt.Degraded, "Degraded"},
		{kt.Failed, "Failed"},
		{kt.Unknown, "Unknown"},
	}

	for _, tc := range tests {
		_, label := healthDisplay(tc.state)
		if !strings.Contains(label, tc.wantLabel) {
			t.Errorf("healthDisplay(%v) label = %q, want it to contain %q", tc.state, label, tc.wantLabel)
		}
	}
}

func TestHealthDisplay_AllStatesCovered(t *testing.T) {
	// Ensure every HealthState returns a non-empty symbol and label
	states := []kt.HealthState{kt.Healthy, kt.Degraded, kt.Failed, kt.Unknown}
	for _, s := range states {
		symbol, label := healthDisplay(s)
		if symbol == "" {
			t.Errorf("healthDisplay(%v) symbol is empty", s)
		}
		if label == "" {
			t.Errorf("healthDisplay(%v) label is empty", s)
		}
	}
}
