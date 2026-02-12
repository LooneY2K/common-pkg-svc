package log

import (
	"fmt"
	"time"

	"github.com/LooneY2K/common-pkg-svc/log/internal/bufferpool"
)

func (l *Logger) logJSON(level Level, msg string, fields ...Field) {
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)

	buf.WriteString(`{"time":"`)
	buf.Write(l.timeFn().AppendFormat(nil, time.RFC3339))
	buf.WriteString(`","level":"`)
	buf.WriteString(level.String())
	buf.WriteString(`","component":"`)
	buf.WriteString(l.component)
	buf.WriteString(`","msg":"`)
	buf.WriteString(msg)
	buf.WriteString(`"`)

	for _, f := range fields {
		buf.WriteString(`,"`)
		buf.WriteString(f.Key)
		buf.WriteString(`":"`)
		buf.WriteString(fmt.Sprint(f.Value))
		buf.WriteString(`"`)
	}

	buf.WriteString("}\n")

	l.out.Write(buf.Bytes())
}
