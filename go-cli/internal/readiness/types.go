package readiness

type Severity string
type Status string
type Level string

const (
	Critical Severity = "critical"
	High     Severity = "high"
	Medium   Severity = "medium"
	Low      Severity = "low"
	Info     Severity = "info"

	Passed  Status = "passed"
	Failed  Status = "failed"
	Skipped Status = "skipped"
	Unknown Status = "unknown"
)

type CheckResult struct {
	ID               string         `json:"id"`
	Title            string         `json:"title"`
	Category         string         `json:"category"`
	Severity         Severity       `json:"severity"`
	Status           Status         `json:"status"`
	Summary          string         `json:"summary"`
	Details          string         `json:"details,omitempty"`
	Remediation      string         `json:"remediation,omitempty"`
	AutoFixAvailable bool           `json:"autoFixAvailable"`
	Evidence         map[string]any `json:"evidence,omitempty"`
}

type Report struct {
	Target      string        `json:"target"`
	Profile     []string      `json:"profile"`
	Score       int           `json:"score"`
	Level       Level         `json:"level"`
	Checks      []CheckResult `json:"checks"`
	GeneratedAt string        `json:"generatedAt"`
}
