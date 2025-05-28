package logger

import (
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

const TRACE_DEPTH int = 10
const PATH bool = false

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

type LogEntry struct {
	Timestamp time.Time
	Level     LogLevel
	Message   string
	File      string
	Function  string
	Line      int
	Trace     []string
	Path      string
}

func (l LogLevel) String() string {
	return [...]string{"DEBUG", "INFO", "WARN", "ERROR", "FATAL"}[l]
}

func Log_dps(level LogLevel, message string) LogEntry {
	pc, file, line, _ := runtime.Caller(2)
	funcName := runtime.FuncForPC(pc).Name()
	trace := make([]string, 0)
	for i := 2; i <= TRACE_DEPTH; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		fn := runtime.FuncForPC(pc).Name()
		traceEntry := fmt.Sprintf("%s:%d %s", file, line, filepath.Base(fn))
		trace = append(trace, traceEntry)
	}

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		File:      filepath.Base(file),
		Function:  filepath.Base(funcName),
		Line:      line,
		Trace:     trace,
		Path:      file,
	}

	if PATH {
		fmt.Printf("\n[PATH] %s:%d",
			entry.Path,
			entry.Line,
		)
	}
	fmt.Printf("\n[%s] %s - %s:%d - %s\n",
		entry.Level,
		entry.Timestamp.Format("2006-01-02 15:04:05"),
		entry.File,
		entry.Line,
		entry.Message,
	)

	if level >= WARN {
		fmt.Println("Stack trace:")
		for i, t := range entry.Trace {
			fmt.Printf("%d: %s\n", i, t)
			if level == WARN && i == 4 {
				break
			}
		}
	}

	return entry
}

func Debug(message string) LogEntry {
	return Log_dps(DEBUG, message)
}

func Info(message string) LogEntry {
	return Log_dps(INFO, message)
}

func Warn(message string) LogEntry {
	return Log_dps(WARN, message)
}

func Error(message string) LogEntry {
	return Log_dps(ERROR, message)
}

func Fatal(message string) LogEntry {
	entry := Log_dps(FATAL, message)
	return entry
}