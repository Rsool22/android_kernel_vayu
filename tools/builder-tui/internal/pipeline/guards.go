package pipeline

import "strings"

// GuardEntry models one row of apply_ksu_guards.py output. The bash
// equivalent split each line into "<file> | <detail>"; we keep the same
// pair plus the original tag (FIX / OK / SKP) so the TUI can colour rows
// per status.
type GuardEntry struct {
	Tag    string // "FIX", "OK", "SKP"
	File   string // body before " |" (or full body for SKP rows)
	Detail string // body after "| "
}

// GuardReport is the parsed output of apply_ksu_guards.py.
type GuardReport struct {
	Fixed   []GuardEntry
	OK      []GuardEntry
	Skipped []GuardEntry
	Summary string // text after "=> Summary: "
}

// Total returns the total entry count across all 3 buckets.
func (g GuardReport) Total() int {
	return len(g.Fixed) + len(g.OK) + len(g.Skipped)
}

// HasFixes is true when at least one [FIX] entry was produced.
func (g GuardReport) HasFixes() bool { return len(g.Fixed) > 0 }

// ParseGuardReport scans apply_ksu_guards.py stdout (combined output) for
// "[FIX] file | detail", "[ OK] file | detail" and "[SKP] detail" rows
// plus the trailing "=> Summary: …" line.
func ParseGuardReport(stdout string) GuardReport {
	var rep GuardReport
	for _, raw := range strings.Split(stdout, "\n") {
		line := strings.TrimLeft(raw, " ")
		switch {
		case strings.HasPrefix(line, "[FIX] "):
			rep.Fixed = append(rep.Fixed, splitGuardBody("FIX", line[len("[FIX] "):]))
		case strings.HasPrefix(line, "[ OK] "):
			rep.OK = append(rep.OK, splitGuardBody("OK", line[len("[ OK] "):]))
		case strings.HasPrefix(line, "[SKP] "):
			rep.Skipped = append(rep.Skipped, splitGuardBody("SKP", line[len("[SKP] "):]))
		case strings.HasPrefix(line, "=> Summary:"):
			rep.Summary = strings.TrimSpace(strings.TrimPrefix(line, "=> Summary:"))
		}
	}
	return rep
}

func splitGuardBody(tag, body string) GuardEntry {
	body = strings.TrimRight(body, " \r")
	if i := strings.Index(body, " | "); i != -1 {
		return GuardEntry{Tag: tag, File: strings.TrimSpace(body[:i]), Detail: strings.TrimSpace(body[i+3:])}
	}
	if i := strings.Index(body, "| "); i != -1 {
		return GuardEntry{Tag: tag, File: strings.TrimSpace(body[:i]), Detail: strings.TrimSpace(body[i+2:])}
	}
	return GuardEntry{Tag: tag, File: strings.TrimSpace(body)}
}
