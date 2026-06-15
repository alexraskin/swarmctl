package ver

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"time"
)

// These are overridden at build time via -ldflags "-X". When unset (e.g. local
// `go run`), Load falls back to the module's embedded build info.
var (
	version   = ""
	commit    = ""
	buildTime = ""
)

func Load() Version {
	v := Version{
		Version:   version,
		GoVersion: runtime.Version(),
		Revision:  commit,
		BuildTime: buildTime,
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		v.GoVersion = info.GoVersion
		if v.Version == "" {
			v.Version = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if v.Revision == "" {
					v.Revision = setting.Value
				}
			case "build.timestamp":
				if v.BuildTime == "" {
					v.BuildTime = setting.Value
				}
			case "vcs.dirty":
				v.Dirty = setting.Value == "true"
			}
		}
	}

	if v.Version == "" {
		v.Version = "devel"
	}
	if v.Revision == "" {
		v.Revision = "unknown"
	}
	if v.BuildTime == "" {
		v.BuildTime = "unknown"
	}

	return v
}

type Version struct {
	Version   string
	GoVersion string
	Revision  string
	BuildTime string
	Dirty     bool
}

func (v Version) Format() string {
	commit := v.Revision
	if len(commit) > 7 {
		commit = commit[:7]
	}

	var buildTimeStr string
	buildTime, err := time.Parse(time.RFC3339, v.BuildTime)
	if err != nil {
		buildTimeStr = "unknown"
	} else {
		buildTimeStr = buildTime.Format(time.ANSIC)
	}

	return fmt.Sprintf("Go Version: %s\nVersion: %s\nCommit: %s\nBuild Time: %s\nOS/Arch: %s/%s\n", v.GoVersion, v.Version, commit, buildTimeStr, runtime.GOOS, runtime.GOARCH)
}
