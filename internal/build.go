package internal

import (
	"runtime/debug"
	"time"
)

var (
	BuildRevision      = "unknown"
	BuildRevisionTime  = time.Time{}
	BuildLocalModified = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			BuildRevision = setting.Value
		case "vcs.time":
			if setting.Value == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, setting.Value)
			if err != nil {
				panic(err)
			}
			BuildRevisionTime = t
		case "vcs.modified":
			BuildLocalModified = setting.Value
		}
	}
}
