// Package naming composes the build artefact filenames used by the
// vayu Bubble Tea builder. It mirrors the bash original:
//
//	[VAYU-AnyMore-Project]
//	[VAYU-AnyMore-Project]-[Kernel-Name=AnyMore-v2.1]
//	[VAYU-AnyMore-Project]-[DEV-ReSukiSU=Manual-Hook]
//	[VAYU-AnyMore-Project]-[DEV-ReSukiSU=Manual-Hook]-(+SuSFS+KPM)
//	-(2025-04-25)-{Build-#42}.zip
//	-(2025-04-25)-Build-#42.log
//
// Naming is deterministic given (project, kernel-name, ksu state, branch,
// extras, date, build number). The bash builder used these exact strings
// so users can move artefacts between TUI implementations transparently.
package naming

import (
	"fmt"
	"strings"
	"time"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
)

// DefaultProject is the prefix used in artefact names.
const DefaultProject = "VAYU-AnyMore"

// Args carries the inputs to all naming helpers.
type Args struct {
	// Project is the project tag inside [Project-Project] brackets.
	Project string
	// KernelName, when non-empty, becomes -[Kernel-Name=<value>].
	KernelName string
	// State / Branch supply the capability/extras tags via features.State.
	State  features.State
	Branch string
	// Date overrides the embedded date stamp; zero value uses time.Now().
	Date time.Time
	// Build is the rolling build counter (from state.ReadBuildNumber).
	Build int
}

func (a Args) project() string {
	if a.Project == "" {
		return DefaultProject
	}
	return a.Project
}

func (a Args) date() string {
	t := a.Date
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("2006-01-02")
}

// LogBase returns the bash `_build_log_base` string:
//
//	[Project-Project]
//	[Project-Project]-[Kernel-Name=K]
//	[Project-Project]-[BTAG-ReSukiSU=Hook-Mode]
//	[Project-Project]-[BTAG-ReSukiSU=Hook-Mode]-(+SuSFS+KPM)
//	-(YYYY-MM-DD)
func LogBase(a Args) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s-Project]", a.project())
	if a.KernelName != "" {
		fmt.Fprintf(&b, "-[Kernel-Name=%s]", a.KernelName)
	}
	if a.State.KSU {
		btag := strings.ToUpper(a.Branch)
		if btag != "DEV" {
			btag = "MAIN"
		}
		hook := "Manual-Hook"
		if a.State.SUSFS {
			hook = "SuSFS-Inline-Hook"
		}
		fmt.Fprintf(&b, "-[%s-ReSukiSU=%s]", btag, hook)

		extras := []string{}
		if a.State.SUSFS {
			extras = append(extras, "SuSFS")
		}
		if a.State.KPM {
			extras = append(extras, "KPM")
		}
		if len(extras) > 0 {
			fmt.Fprintf(&b, "-(+%s)", strings.Join(extras, "+"))
		}
	}
	fmt.Fprintf(&b, "-(%s)", a.date())
	return b.String()
}

// ZipName returns the AnyKernel3 zip filename for a given build.
//
//	<LogBase>-{Build-#42}.zip
func ZipName(a Args) string {
	return fmt.Sprintf("%s-{Build-#%d}.zip", LogBase(a), a.Build)
}

// SuccessLogName returns the success log filename for build N.
//
//	<LogBase>-Build-#42.log
func SuccessLogName(a Args) string {
	return fmt.Sprintf("%s-Build-#%d.log", LogBase(a), a.Build)
}

// FailLogName returns the static failure log name (overwritten each
// failed build, like the bash original).
//
//	[Project-Project]-FAIL.log
func FailLogName(project string) string {
	if project == "" {
		project = DefaultProject
	}
	return fmt.Sprintf("[%s-Project]-FAIL.log", project)
}

// CancelLogName returns the static cancellation log name.
//
//	[Project-Project]-CANCEL.log
func CancelLogName(project string) string {
	if project == "" {
		project = DefaultProject
	}
	return fmt.Sprintf("[%s-Project]-CANCEL.log", project)
}

// CapTag wraps features.State.CapTag for callers that already have a
// naming.Args and want a one-call helper.
func CapTag(a Args) string { return a.State.CapTag(a.Branch) }

// ExtTag wraps features.State.ExtTag.
func ExtTag(a Args) string { return a.State.ExtTag() }
