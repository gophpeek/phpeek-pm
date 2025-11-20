package logger

import (
	"bufio"
	"bytes"
	"log/slog"
)

// ProcessWriter captures process output and logs it with structured metadata
type ProcessWriter struct {
	Logger     *slog.Logger
	InstanceID string
	Stream     string // stdout or stderr
	buffer     bytes.Buffer
}

// Write implements io.Writer
func (pw *ProcessWriter) Write(p []byte) (n int, err error) {
	pw.buffer.Write(p)

	// Process complete lines
	scanner := bufio.NewScanner(&pw.buffer)
	var remaining bytes.Buffer

	for scanner.Scan() {
		line := scanner.Text()

		// Log with structured metadata
		pw.Logger.Info(line,
			"instance_id", pw.InstanceID,
			"stream", pw.Stream,
		)
	}

	// Keep incomplete line in buffer
	if pw.buffer.Len() > 0 {
		remaining.Write(pw.buffer.Bytes())
	}
	pw.buffer = remaining

	return len(p), nil
}
