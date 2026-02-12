package log

type Level uint8

const (
	Debug Level = iota
	Info
	Warn
	Error
)

var levelStrings = [...]string{
	"DEBUG",
	"INFO ",
	"WARN ",
	"ERROR",
}

func (l Level) String() string {
	if int(l) >= len(levelStrings) {
		return "UNKNOWN"
	}
	return levelStrings[l]
}
