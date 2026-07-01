package runner

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"infer-tester/internal/config"
)

var ErrSkip = errors.New("SKIP")

func Skip(reason string) (string, error) { return reason, ErrSkip }

type Status string

const (
	StatusPass Status = "PASS"
	StatusFail Status = "FAIL"
	StatusSkip Status = "SKIP"
)

type TestCase struct {
	Name        string
	Description string
	Run         func() (detail string, err error)
}

type Suite struct {
	Name        string
	Description string
	Cases       []TestCase
}

type CaseResult struct {
	Name       string `json:"name"`
	Status     Status `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
	Error      string `json:"error,omitempty"`
}

type SuiteResult struct {
	Name    string       `json:"name"`
	Passed  int          `json:"passed"`
	Failed  int          `json:"failed"`
	Skipped int          `json:"skipped"`
	Cases   []CaseResult `json:"cases"`
}

type Summary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

type Report struct {
	Timestamp string        `json:"timestamp"`
	Server    string        `json:"server"`
	Model     string        `json:"model"`
	Duration  string        `json:"duration"`
	Summary   Summary       `json:"summary"`
	Suites    []SuiteResult `json:"suites"`
}

type Runner struct {
	cfg    *config.Config
	suites []Suite
}

func New(cfg *config.Config) *Runner { return &Runner{cfg: cfg} }

func (r *Runner) Add(s Suite) { r.suites = append(r.suites, s) }

func (r *Runner) List() {
	total := 0
	for _, s := range r.suites {
		fmt.Printf("\n[%s]  (%d cases)\n", s.Name, len(s.Cases))
		if s.Description != "" {
			fmt.Printf("  # %s\n", s.Description)
		}
		for _, tc := range s.Cases {
			fmt.Printf("  %-52s %s\n", tc.Name, tc.Description)
			total++
		}
	}
	fmt.Printf("\nTotal: %d cases in %d suites\n", total, len(r.suites))
}

func (r *Runner) Run(suiteFilter, caseFilter []string) Report {
	start := time.Now()
	report := Report{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Server:    r.cfg.Server.BaseURL,
		Model:     r.cfg.Model.Name,
	}

	matchAny := func(name string, filters []string) bool {
		if len(filters) == 0 {
			return true
		}
		lower := strings.ToLower(name)
		for _, f := range filters {
			if strings.Contains(lower, strings.ToLower(f)) {
				return true
			}
		}
		return false
	}

	caseEnabled := func(suiteName, caseName string) bool {
		if suiteMap, ok := r.cfg.Cases[suiteName]; ok {
			if enabled, ok := suiteMap[caseName]; ok {
				return enabled
			}
		}
		return true
	}

	for _, suite := range r.suites {
		if !matchAny(suite.Name, suiteFilter) {
			continue
		}
		fmt.Printf("\n[%s]\n", suite.Name)
		sr := SuiteResult{Name: suite.Name}

		for _, tc := range suite.Cases {
			if !matchAny(tc.Name, caseFilter) {
				continue
			}
			if !caseEnabled(suite.Name, tc.Name) {
				sr.Cases = append(sr.Cases, CaseResult{Name: tc.Name, Status: StatusSkip})
				sr.Skipped++
				fmt.Printf("  - %-52s disabled\n", tc.Name)
				continue
			}
			t0 := time.Now()
			detail, err := tc.Run()
			dur := time.Since(t0)

			cr := CaseResult{
				Name:       tc.Name,
				DurationMs: dur.Milliseconds(),
				Detail:     detail,
			}
			switch {
			case err == nil:
				cr.Status = StatusPass
				sr.Passed++
				fmt.Printf("  ✓ %-52s %dms", tc.Name, dur.Milliseconds())
				if r.cfg.Output.Verbose && detail != "" {
					fmt.Printf("  %s", detail)
				}
				fmt.Println()
			case errors.Is(err, ErrSkip):
				cr.Status = StatusSkip
				sr.Skipped++
				fmt.Printf("  - %-52s skipped: %s\n", tc.Name, detail)
			default:
				cr.Status = StatusFail
				cr.Error = err.Error()
				sr.Failed++
				fmt.Printf("  ✗ %-52s %dms  %v\n", tc.Name, dur.Milliseconds(), err)
			}
			sr.Cases = append(sr.Cases, cr)
		}
		report.Suites = append(report.Suites, sr)
		report.Summary.Total += sr.Passed + sr.Failed + sr.Skipped
		report.Summary.Passed += sr.Passed
		report.Summary.Failed += sr.Failed
		report.Summary.Skipped += sr.Skipped
	}

	report.Duration = time.Since(start).Round(time.Millisecond).String()
	return report
}
