package elog

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/LooneY2K/common-pkg-svc/log/internal/bufferpool"
)

func (l *Logger) logPretty(level Level, msg string, fields ...Field) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)

	buf.Write(l.timeFn().AppendFormat(nil, "15:04:05"))
	buf.WriteString("  ")
	buf.WriteString(levelStrings[level])
	buf.WriteString("  ")

	if l.component != "" {
		buf.WriteString(padRight(l.component, 10))
		buf.WriteString("  ")
	}

	buf.WriteString(msg)

	if len(fields) == 1 {
		buf.WriteString(" ")
		appendField(buf, fields[0])
		buf.WriteByte('\n')
		l.out.Write(buf.Bytes())
		return
	}

	buf.WriteByte('\n')

	if len(fields) > 0 {
		buf.WriteString("            ")
		for i, f := range fields {
			appendField(buf, f)
			if i != len(fields)-1 {
				buf.WriteString("  ")
			}
		}
		buf.WriteByte('\n')
	}

	l.out.Write(buf.Bytes())
}

func appendField(buf *bytes.Buffer, f Field) {
	buf.WriteString(f.Key)
	buf.WriteByte('=')

	switch v := f.Value.(type) {
	case string:
		buf.WriteString(v)
	case int:
		buf.Write(strconv.AppendInt(nil, int64(v), 10))
	case int64:
		buf.Write(strconv.AppendInt(nil, v, 10))
	case time.Duration:
		buf.WriteString(v.String())
	case error:
		buf.WriteString(v.Error())
	default:
		buf.WriteString(toString(v))
	}
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}

func toString(v any) string {
	return strconv.Quote(fmt.Sprint(v))
}
