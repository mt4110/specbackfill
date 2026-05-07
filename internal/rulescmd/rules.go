package rulescmd

import (
	"fmt"
	"io"

	"github.com/mt4110/specbackfill/internal/rules"
)

func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeUsage(stderr)
		return 2
	}

	switch args[0] {
	case "list":
		if len(args) != 1 {
			return writeError(stderr, "rules list does not accept positional arguments")
		}
		return runList(stdout, stderr)
	case "show":
		if len(args) != 2 {
			return writeError(stderr, "usage: specbackfill rules show <RULE_ID>")
		}
		return runShow(stdout, stderr, args[1])
	default:
		return writeError(stderr, fmt.Sprintf("unknown rules command %q", args[0]))
	}
}

func runList(stdout, stderr io.Writer) int {
	if _, err := fmt.Fprintln(stdout, "Rule    Severity  Description"); err != nil {
		return writeError(stderr, fmt.Sprintf("render rules list: %v", err))
	}
	for _, info := range rules.Catalog() {
		if _, err := fmt.Fprintf(stdout, "%-7s %-9s %s\n", info.ID, info.DefaultSeverity, info.ListDescription); err != nil {
			return writeError(stderr, fmt.Sprintf("render rules list: %v", err))
		}
	}
	return 0
}

func runShow(stdout, stderr io.Writer, ruleID string) int {
	info, ok := rules.FindRuleInfo(ruleID)
	if !ok {
		return writeError(stderr, fmt.Sprintf("unknown rule ID %q", ruleID))
	}

	if _, err := fmt.Fprintf(stdout, "%s: %s\n\n", info.ID, info.Title); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if _, err := fmt.Fprintf(stdout, "Default severity: %s\n\n", info.DefaultSeverity); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if err := writeBullets(stdout, "What triggers it:", info.TriggerBullets); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if err := writeBullets(stdout, "Expected companions:", info.ExpectedCompanions); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	if err := writeBullets(stdout, "Does not report on:", info.DoesNotReportOn); err != nil {
		return writeError(stderr, fmt.Sprintf("render rule detail: %v", err))
	}
	return 0
}

func writeBullets(w io.Writer, heading string, bullets []string) error {
	if _, err := fmt.Fprintln(w, heading); err != nil {
		return err
	}
	for _, bullet := range bullets {
		if _, err := fmt.Fprintf(w, "  - %s\n", bullet); err != nil {
			return err
		}
	}
	return nil
}

func writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: specbackfill rules list")
	fmt.Fprintln(w, "       specbackfill rules show <RULE_ID>")
}

func writeError(w io.Writer, message string) int {
	fmt.Fprintf(w, "error: %s\n", message)
	return 2
}
