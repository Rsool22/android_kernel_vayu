package naming

import (
	"testing"
	"time"

	"github.com/Rsool22/android_kernel_vayu/tools/builder-tui/internal/features"
)

var fixedDate = time.Date(2025, 4, 25, 0, 0, 0, 0, time.UTC)

func base(s features.State, branch, kname string, build int) Args {
	return Args{
		KernelName: kname,
		State:      s,
		Branch:     branch,
		Date:       fixedDate,
		Build:      build,
	}
}

func TestLogBaseVanilla(t *testing.T) {
	got := LogBase(base(features.State{}, "", "", 1))
	want := "[VAYU-AnyMore-Project]-(2025-04-25)"
	if got != want {
		t.Errorf("LogBase vanilla =\n got %q\nwant %q", got, want)
	}
}

func TestLogBaseKSUMain(t *testing.T) {
	st := features.State{KSU: true, DriverPresent: true}
	got := LogBase(base(st, "main", "", 1))
	want := "[VAYU-AnyMore-Project]-[MAIN-ReSukiSU=Manual-Hook]-(2025-04-25)"
	if got != want {
		t.Errorf("LogBase KSU/main =\n got %q\nwant %q", got, want)
	}
}

func TestLogBaseKSUDevSusfsKpm(t *testing.T) {
	st := features.State{KSU: true, SUSFS: true, KPM: true, DriverPresent: true}
	got := LogBase(base(st, "dev", "AnyMore-v2.1", 7))
	want := "[VAYU-AnyMore-Project]-[Kernel-Name=AnyMore-v2.1]-[DEV-ReSukiSU=SuSFS-Inline-Hook]-(+SuSFS+KPM)-(2025-04-25)"
	if got != want {
		t.Errorf("LogBase full =\n got %q\nwant %q", got, want)
	}
}

func TestZipName(t *testing.T) {
	st := features.State{KSU: true, KPM: true, DriverPresent: true}
	got := ZipName(base(st, "main", "", 42))
	want := "[VAYU-AnyMore-Project]-[MAIN-ReSukiSU=Manual-Hook]-(+KPM)-(2025-04-25)-{Build-#42}.zip"
	if got != want {
		t.Errorf("ZipName =\n got %q\nwant %q", got, want)
	}
}

func TestSuccessLogName(t *testing.T) {
	st := features.State{}
	got := SuccessLogName(base(st, "", "", 9))
	want := "[VAYU-AnyMore-Project]-(2025-04-25)-Build-#9.log"
	if got != want {
		t.Errorf("SuccessLogName =\n got %q\nwant %q", got, want)
	}
}

func TestStaticLogNames(t *testing.T) {
	if got := FailLogName(""); got != "[VAYU-AnyMore-Project]-FAIL.log" {
		t.Errorf("FailLogName default = %q", got)
	}
	if got := CancelLogName("MyProj"); got != "[MyProj-Project]-CANCEL.log" {
		t.Errorf("CancelLogName custom = %q", got)
	}
}

func TestProjectOverride(t *testing.T) {
	a := base(features.State{}, "", "", 1)
	a.Project = "Custom"
	got := LogBase(a)
	want := "[Custom-Project]-(2025-04-25)"
	if got != want {
		t.Errorf("LogBase project override =\n got %q\nwant %q", got, want)
	}
}
