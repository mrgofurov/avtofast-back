package logger

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "DEBUG"
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
)

type LogEntry struct {
	Timestamp  string         `json:"timestamp"`
	Level      Level          `json:"level"`
	Message    string         `json:"message"`
	RequestId  string         `json:"requestId,omitempty"`
	Endpoint   string         `json:"endpoint,omitempty"`
	Method     string         `json:"method,omitempty"`
	Status     int            `json:"status,omitempty"`
	LatencyMs  float64        `json:"latencyMs,omitempty"`
	UserId     string         `json:"userId,omitempty"`
	ErrorCode  string         `json:"errorCode,omitempty"`
	ErrorMsg   string         `json:"errorMsg,omitempty"`
	Extra      map[string]any `json:"extra,omitempty"`
}

type Logger struct {
	mu     sync.Mutex
	out    io.Writer
	minLvl Level
}

var DefaultLogger = New(os.Stdout, LevelInfo)

func New(out io.Writer, minLvl Level) *Logger {
	return &Logger{
		out:    out,
		minLvl: minLvl,
	}
}

func (l *Logger) Log(entry LogEntry) {
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	data, _ := json.Marshal(entry)
	data = append(data, '\n')
	_, _ = l.out.Write(data)
}

func (l *Logger) Info(msg string, extra ...map[string]any) {
	var ext map[string]any
	if len(extra) > 0 {
		ext = extra[0]
	}
	l.Log(LogEntry{
		Level:   LevelInfo,
		Message: msg,
		Extra:   ext,
	})
}

func (l *Logger) Error(msg string, err error, extra ...map[string]any) {
	var ext map[string]any
	if len(extra) > 0 {
		ext = extra[0]
	}
	errStr := ""
	if err != nil {
		errStr = err.Error()
	}
	l.Log(LogEntry{
		Level:    LevelError,
		Message:  msg,
		ErrorMsg: errStr,
		Extra:    ext,
	})
}
