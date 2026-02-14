package zap

type Config struct {
	Level       string // debug, info, warn, error
	Environment string // development or production
	ServiceName string
	LogFile     string // optional file path
}
