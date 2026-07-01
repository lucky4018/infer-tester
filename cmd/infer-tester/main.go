package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"infer-tester/internal/client"
	"infer-tester/internal/config"
	"infer-tester/internal/report"
	"infer-tester/internal/runner"
	"infer-tester/internal/suite"
)

func main() {
	configPath := flag.String("config", "config.json", "config file path (default: config.json in current directory)")
	reportPath := flag.String("report", "", "report output path (overrides config)")
	onlySuites := flag.String("suite", "", "comma-separated suite name fragments to run (e.g. smoke,api)")
	onlyCases := flag.String("case", "", "comma-separated case name fragments to run (e.g. streaming,health)")
	listOnly := flag.Bool("list", false, "list all suites and cases without running")
	verbose := flag.Bool("v", false, "print test detail on pass")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if *reportPath != "" {
		cfg.Output.ReportFile = *reportPath
	}
	if *verbose {
		cfg.Output.Verbose = true
	}

	splitFilter := func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, ",") {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	suiteFilter := splitFilter(*onlySuites)
	caseFilter := splitFilter(*onlyCases)

	fmt.Printf("infer-tester\n")
	fmt.Printf("Server : %s\n", cfg.Server.BaseURL)

	c := client.New(cfg)
	original := cfg.Model.Name
	cfg.Model.Name = resolveModel(c, cfg.Model.Name)
	switch {
	case original == "":
		fmt.Printf("Model  : %s (auto-resolved)\n", cfg.Model.Name)
	case cfg.Model.Name != original:
		fmt.Printf("Model  : %s (configured: %s not found)\n", cfg.Model.Name, original)
	default:
		fmt.Printf("Model  : %s\n", cfg.Model.Name)
	}

	r := runner.New(cfg)
	r.Add(suite.Smoke(c, cfg))
	r.Add(suite.API(c, cfg))
	r.Add(suite.Sampling(c, cfg))
	r.Add(suite.Features(c, cfg))
	r.Add(suite.Performance(c, cfg))
	r.Add(suite.Concurrency(c, cfg))

	if *listOnly {
		r.List()
		return
	}

	rpt := r.Run(suiteFilter, caseFilter)
	report.PrintSummary(rpt)

	if cfg.Output.ReportFile != "" {
		if err := report.Write(cfg.Output.ReportFile, rpt); err != nil {
			fmt.Fprintf(os.Stderr, "write report: %v\n", err)
		} else {
			md, js := report.Paths(cfg.Output.ReportFile)
			fmt.Printf("\nReports written:\n  %s\n  %s\n", md, js)
		}
	}

	if rpt.Summary.Failed > 0 {
		os.Exit(1)
	}
}

// resolveModel returns the model name to use.
// If configured name is empty or not found in /v1/models, falls back to the first available model.
func resolveModel(c *client.Client, configured string) string {
	raw, err := c.Get(context.Background(), "/v1/models")
	if err != nil || raw.Status != 200 {
		// Can't reach /v1/models; proceed with whatever was configured (may be empty)
		return configured
	}

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw.Body, &resp); err != nil || len(resp.Data) == 0 {
		return configured
	}

	if configured == "" {
		return resp.Data[0].ID
	}

	for _, m := range resp.Data {
		if m.ID == configured {
			return configured
		}
	}

	// Configured name not in list — fall back to first model
	return resp.Data[0].ID
}
