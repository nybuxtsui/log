package log

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type buffPool struct {
	pool sync.Pool
}

var buffs = &buffPool{
	pool: sync.Pool{
		New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0))
		},
	},
}

func (b *buffPool) get() *bytes.Buffer {
	return b.pool.Get().(*bytes.Buffer)
}

func (b *buffPool) put(buf *bytes.Buffer) {
	buf.Reset()
	b.pool.Put(buf)
}

// DefaultFormatter 默认格式化
type DefaultFormatter struct {
	format string
}

func getLevelStr(level int) byte {
	switch level {
	case DEBUG:
		return 'D'
	case INFO:
		return 'I'
	case WARN:
		return 'W'
	case ERROR:
		return 'E'
	case FATAL:
		return 'F'
	default:
		fmt.Printf("ERROR: logger level unknown: %v\n", level)
		return 'I'
	}
}

func getLevelStr2(level int) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		fmt.Printf("ERROR: logger level unknown: %v\n", level)
		return "INFO"
	}
}

// Format 格式化
func (format *DefaultFormatter) Format(level int, msg string) *bytes.Buffer {
	buff := buffs.get()
	t := time.Now()
	dateTimeStr := t.Format("2006-01-02 15:04:05.000")
	buff.WriteString(dateTimeStr)
	buff.WriteString(" ")
	buff.WriteString(getLevelStr2(level))
	_, file, line, ok := runtime.Caller(3)
	if ok {
		buff.WriteByte(' ')
		var i = len(file) - 2
		for ; i >= 0; i-- {
			if file[i] == '/' {
				i++
				break
			}
		}
		buff.WriteString(file[i:])
		buff.WriteByte(':')
		buff.WriteString(strconv.FormatInt(int64(line), 10))
	}
	buff.WriteString("] ")
	buff.WriteString(msg)
	buff.WriteByte('\n')
	return buff
}
