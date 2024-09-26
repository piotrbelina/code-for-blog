package main

import (
	"log/slog"
	"os"
	"runtime"
	"runtime/debug"
)

var GitCommit = "NOCOMMIT"
var GoVersion = runtime.Version()
var BuildDate = ""

func initVersion() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	modified := false
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			GitCommit = setting.Value
		case "vcs.time":
			BuildDate = setting.Value
		case "vcs.modified":
			modified = true
		}
	}
	if modified {
		GitCommit += "+CHANGES"
	}
}

func main() {
	initVersion()

	handler := slog.NewTextHandler(os.Stdout, nil)
	logger := slog.New(handler)

	logger = logger.With(slog.String("revision", GitCommit), slog.String("go_version", GoVersion), slog.String("build_date", BuildDate))

	logger.Info("hello")
}
