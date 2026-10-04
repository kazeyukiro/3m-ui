// Package panellog keeps a ring buffer of panel process logs (stdlib log
// output) so the UI "Logs" page can show panel runtime lines alongside Mihomo.
package panellog

import (
	"strings"
	"sync"
	"time"
)

const maxLines = 500

var (
	mu    sync.Mutex
	lines []string
)

// Writer implements io.Writer for log.SetOutput (use with io.MultiWriter(os.Stderr, …)).
type Writer struct{}

func (Writer) Write(p []byte) (int, error) {
	Append(string(p))
	return len(p), nil
}

// Append stores one or more log lines (split on newlines).
func Append(chunk string) {
	chunk = strings.ReplaceAll(chunk, "\r\n", "\n")
	parts := strings.Split(chunk, "\n")
	mu.Lock()
	defer mu.Unlock()
	for _, line := range parts {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Avoid double-prefix if caller already included a timestamp.
		stored := line
		if !looksLikeRFC3339Prefix(line) {
			stored = time.Now().Format(time.RFC3339) + " [panel] " + line
		} else if !strings.Contains(line, "[panel]") {
			// Insert [panel] after timestamp for UI filter/level hints.
			if idx := strings.IndexByte(line, ' '); idx > 0 {
				stored = line[:idx] + " [panel] " + strings.TrimSpace(line[idx+1:])
			}
		}
		lines = append(lines, stored)
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
}

func looksLikeRFC3339Prefix(line string) bool {
	idx := strings.IndexByte(line, ' ')
	if idx < 20 {
		return false
	}
	_, err := time.Parse(time.RFC3339, line[:idx])
	return err == nil
}

// Lines returns a copy of buffered panel log lines (oldest first).
func Lines() []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}
