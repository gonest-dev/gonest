package app

import "gonest.dev/gonest/internal/appoptions"

// AppOptions/LogLevel/OnListen (and the LogLevelXxx consts) are plain type
// aliases of internal/appoptions's own definitions -- moved out of this
// package (see internal/appoptions's own package doc comment for why:
// internal/adapter/fiber needs the type too, and this package already
// imports internal/adapter/fiber via testapp.go, so AppOptions living here
// would be an import cycle). Every existing call site inside this package
// (NewApp/MustNewApp/HttpAdapter/etc, all still bare "AppOptions") keeps
// working unchanged -- a type alias is not a new type.
type AppOptions = appoptions.AppOptions
type LogLevel = appoptions.LogLevel
type OnListen = appoptions.OnListen

const (
	LogLevelError   = appoptions.LogLevelError
	LogLevelWarn    = appoptions.LogLevelWarn
	LogLevelLog     = appoptions.LogLevelLog
	LogLevelDebug   = appoptions.LogLevelDebug
	LogLevelVerbose = appoptions.LogLevelVerbose
)
