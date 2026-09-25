package logger

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Prompt-input dedicated daily file.
//
// The input-only prompt log is deliberately separated from the main
// zap log file: it is high-volume, contains raw user input, and must be
// easy to retain/purge per day. One JSON object per line:
//
//	<logdir>/prompt-input-YYYY-MM-DD.log
//
// The file is never forwarded to the ops DB sink, so nothing lands in
// ops_system_logs or any other data table.
//
// Locking: promptInputMu and the logger mu are never held at the same
// time. The directory/path/retention are always resolved before taking
// promptInputMu, so a concurrent Init/Reconfigure cannot deadlock with
// the write path; a config change simply makes the next write reopen
// the handle at the new path.

const promptInputFilenamePrefix = "prompt-input-"

var (
	promptInputMu          sync.Mutex
	promptInputFile        *os.File
	promptInputCurrentPath string
	promptInputDirOverride string
)

// SetPromptInputDirOverride redirects the daily prompt-input file to dir.
// It is intended for tests; production code leaves it empty so the file
// lands next to the main log file.
func SetPromptInputDirOverride(dir string) {
	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	promptInputDirOverride = dir
	if promptInputFile != nil {
		_ = promptInputFile.Close()
		promptInputFile = nil
		promptInputCurrentPath = ""
	}
}

// PromptInputLogDir returns the directory holding the daily files.
func PromptInputLogDir() string {
	promptInputMu.Lock()
	override := promptInputDirOverride
	promptInputMu.Unlock()
	if override != "" {
		return override
	}
	return mainLogDir()
}

// mainLogDir reads the configured main log file path under the logger lock
// so concurrent Init/Reconfigure cannot race with prompt-input writes.
func mainLogDir() string {
	mu.RLock()
	filePath := initOptions.Output.FilePath
	mu.RUnlock()
	return filepath.Dir(resolveLogFilePath(filePath))
}

// promptInputRetentionDays mirrors the main log rotation retention so old
// daily prompt-input files are purged on the same schedule. Zero or
// negative means keep (same semantics as lumberjack MaxAge).
func promptInputRetentionDays() int {
	mu.RLock()
	days := initOptions.Rotation.MaxAgeDays
	mu.RUnlock()
	return days
}

// PromptInputFilename returns the full path of today's daily file.
func PromptInputFilename() string {
	return PromptInputFilenameForDate(time.Now())
}

// PromptInputFilenameForDate returns the daily file path for the given time.
func PromptInputFilenameForDate(t time.Time) string {
	return filepath.Join(PromptInputLogDir(), promptInputFilenamePrefix+t.Format("2006-01-02")+".log")
}

// AppendPromptInputLine appends one JSON line (without trailing newline
// required) to today's daily file, reopening the handle on date or
// directory change and purging expired daily files.
func AppendPromptInputLine(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	// Resolve everything needing other locks before taking promptInputMu.
	path := PromptInputFilename()
	dir := filepath.Dir(path)
	retention := promptInputRetentionDays()

	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	if promptInputFile == nil || promptInputCurrentPath != path {
		if promptInputFile != nil {
			_ = promptInputFile.Close()
			promptInputFile = nil
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		promptInputFile = f
		promptInputCurrentPath = path
		purgeOldPromptInputFiles(dir, retention)
	}
	buf := make([]byte, 0, len(line)+1)
	buf = append(buf, line...)
	if line[len(line)-1] != '\n' {
		buf = append(buf, '\n')
	}
	_, err := promptInputFile.Write(buf)
	return err
}

// ClosePromptInputFile releases the cached daily file handle.
func ClosePromptInputFile() {
	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	if promptInputFile != nil {
		_ = promptInputFile.Close()
		promptInputFile = nil
		promptInputCurrentPath = ""
	}
}

// purgeOldPromptInputFiles deletes daily files older than retention days.
// Callers must hold promptInputMu; it touches only the filesystem.
func purgeOldPromptInputFiles(dir string, retention int) {
	if retention <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retention).Format("2006-01-02")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, promptInputFilenamePrefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		date := strings.TrimSuffix(strings.TrimPrefix(name, promptInputFilenamePrefix), ".log")
		if len(date) != len("2006-01-02") || date >= cutoff {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
