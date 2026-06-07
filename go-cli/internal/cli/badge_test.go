package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBadgeColor(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "brightgreen"},
		{95, "brightgreen"},
		{90, "brightgreen"},
		{89, "green"},
		{70, "green"},
		{69, "yellow"},
		{50, "yellow"},
		{49, "orange"},
		{30, "orange"},
		{29, "red"},
		{0, "red"},
	}
	for _, tt := range tests {
		got := badgeColor(tt.score)
		if got != tt.want {
			t.Errorf("badgeColor(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestBadgeURL(t *testing.T) {
	url := badgeURL(87)
	if !strings.Contains(url, "img.shields.io") {
		t.Errorf("expected shields.io URL, got %q", url)
	}
	if !strings.Contains(url, "87%25") {
		t.Errorf("expected score 87%%25 in URL, got %q", url)
	}
	if !strings.Contains(url, "green") {
		t.Errorf("expected green color in URL, got %q", url)
	}
}

func TestBadgeSVG(t *testing.T) {
	svg := badgeSVG(92)
	if !strings.HasPrefix(svg, "<svg") {
		t.Errorf("expected SVG output, got %q", svg[:50])
	}
	if !strings.Contains(svg, "92%") {
		t.Errorf("expected score 92%% in SVG, got %q", svg)
	}
	if !strings.Contains(svg, "#4c1") {
		t.Errorf("expected brightgreen hex #4c1 in SVG, got %q", svg)
	}
}

func TestScoreFromReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	report := map[string]any{"score": 75, "level": "almost-ready"}
	data, _ := json.Marshal(report)
	os.WriteFile(path, data, 0o644)

	score, err := scoreFromReport(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 75 {
		t.Errorf("scoreFromReport = %d, want 75", score)
	}
}

func TestScoreFromReportInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	os.WriteFile(path, []byte(`not json`), 0o644)

	_, err := scoreFromReport(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestScoreFromReportMissing(t *testing.T) {
	_, err := scoreFromReport("/nonexistent/report.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSvgEscape(t *testing.T) {
	got := svgEscape(`<script>"&</script>`)
	if strings.Contains(got, "<") || strings.Contains(got, ">") || strings.Contains(got, "\"") {
		t.Errorf("svgEscape did not properly escape: %q", got)
	}
	if !strings.Contains(got, "&lt;") || !strings.Contains(got, "&gt;") || !strings.Contains(got, "&quot;") {
		t.Errorf("svgEscape missing expected entities: %q", got)
	}
}
