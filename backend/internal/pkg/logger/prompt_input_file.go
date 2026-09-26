package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Prompt-input dedicated daily file, grouped by username.
//
// The input-only prompt log is deliberately separated from the main
// zap log file: it is high-volume, contains raw user input, and must be
// easy to retain/purge per day and per user. One JSON object per line:
//
//	<logdir>/<username>/YYYY-MM-DD.log
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
	// promptInputFiles caches one open handle per daily file path so
	// concurrent users (董经武, 李泽阳, ...) each append to their own
	// <logdir>/<username>/YYYY-MM-DD.log without reopening per line.
	promptInputFiles       = map[string]*os.File{}
	promptInputDirOverride string
)

// SetPromptInputDirOverride redirects the daily prompt-input file to dir.
// It is intended for tests; production code leaves it empty so the file
// lands next to the main log file.
func SetPromptInputDirOverride(dir string) {
	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	promptInputDirOverride = dir
	closePromptInputFilesLocked()
}

var promptInputMu sync.Mutex

// PromptInputLogDir returns the base directory holding per-user folders.
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

// SanitizePromptInputUsername maps a display username (e.g. 钉钉姓名) to a
// safe single path segment. It preserves CJK characters and only strips
// path separators, NUL/control characters and traversal segments. Empty
// results fall back to unknown (optionally suffixed with user_id).
func SanitizePromptInputUsername(name string, userID int64) string {
	s := strings.TrimSpace(name)
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "\x00", "")
	// Drop other control characters.
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if runes := []rune(s); len(runes) > 64 {
		s = strings.TrimSpace(string(runes[:64]))
	}
	s = strings.Trim(s, ".")
	if s == "" || s == "." || s == ".." {
		if userID > 0 {
			return fmt.Sprintf("unknown-user-%d", userID)
		}
		return "unknown"
	}
	return s
}

// PromptInputUserDir returns <logdir>/<username>/ for the given user.
func PromptInputUserDir(username string, userID int64) string {
	return filepath.Join(PromptInputLogDir(), SanitizePromptInputUsername(username, userID))
}

// PromptInputFilename returns the full path of today's daily file.
// Deprecated: per-user layout is used instead; kept for compatibility.
func PromptInputFilename() string {
	return PromptInputFilenameForDate(time.Now())
}

// PromptInputFilenameForDate returns the legacy flat daily file path.
// Deprecated: per-user layout is used instead; kept for compatibility.
func PromptInputFilenameForDate(t time.Time) string {
	return filepath.Join(PromptInputLogDir(), promptInputFilenamePrefix+t.Format("2006-01-02")+".log")
}

// PromptInputFilenameForUser returns <logdir>/<username>/YYYY-MM-DD.log.
func PromptInputFilenameForUser(username string, userID int64, t time.Time) string {
	return filepath.Join(PromptInputUserDir(username, userID), t.Format("2006-01-02")+".log")
}

// AppendPromptInputLine appends one JSON line to the legacy flat daily
// file. It is kept for compatibility; new code should use
// AppendPromptInputLineForUser.
func AppendPromptInputLine(line []byte) error {
	if len(line) == 0 {
		return nil
	}
	// Resolve everything needing other locks before taking promptInputMu.
	path := PromptInputFilename()
	dir := filepath.Dir(path)
	retention := promptInputRetentionDays()
	base := PromptInputLogDir()

	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	f, err := openPromptInputFileLocked(path, dir)
	if err != nil {
		return err
	}
	purgeOldPromptInputFiles(base, retention)
	return writePromptInputLine(f, line)
}

// AppendPromptInputLineForUser appends one JSON line (without trailing
// newline required) to today's <logdir>/<username>/YYYY-MM-DD.log,
// reopening handles on date/directory/user change and purging expired
// daily files.
func AppendPromptInputLineForUser(username string, userID int64, line []byte) error {
	if len(line) == 0 {
		return nil
	}
	// Resolve everything needing other locks before taking promptInputMu.
	now := time.Now()
	path := PromptInputFilenameForUser(username, userID, now)
	dir := filepath.Dir(path)
	retention := promptInputRetentionDays()
	base := PromptInputLogDir()

	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	f, err := openPromptInputFileLocked(path, dir)
	if err != nil {
		return err
	}
	purgeOldPromptInputFiles(base, retention)
	return writePromptInputLine(f, line)
}

// openPromptInputFileLocked returns the cached handle for path, opening it
// on first use. Callers must hold promptInputMu.
func openPromptInputFileLocked(path, dir string) (*os.File, error) {
	if f, ok := promptInputFiles[path]; ok && f != nil {
		return f, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	// Bound the cache: one entry per active (user, date) path. Evict the
	// oldest entry when the map grows beyond a sane bound so a large user
	// base cannot pin unbounded file descriptors.
	const maxPromptInputOpenFiles = 256
	if len(promptInputFiles) >= maxPromptInputOpenFiles {
		for oldPath, oldFile := range promptInputFiles {
			if oldPath == path {
				continue
			}
			_ = oldFile.Close()
			delete(promptInputFiles, oldPath)
			break
		}
	}
	promptInputFiles[path] = f
	return f, nil
}

func writePromptInputLine(f *os.File, line []byte) error {
	buf := make([]byte, 0, len(line)+1)
	buf = append(buf, line...)
	if line[len(line)-1] != '\n' {
		buf = append(buf, '\n')
	}
	_, err := f.Write(buf)
	return err
}

// ClosePromptInputFile releases all cached daily file handles.
func ClosePromptInputFile() {
	promptInputMu.Lock()
	defer promptInputMu.Unlock()
	closePromptInputFilesLocked()
}

func closePromptInputFilesLocked() {
	for path, f := range promptInputFiles {
		if f != nil {
			_ = f.Close()
		}
		delete(promptInputFiles, path)
	}
}

// purgeOldPromptInputFiles deletes daily files older than retention days,
// both the legacy flat files in base and per-user <base>/<user>/*.log.
// Callers must hold promptInputMu; it touches only the filesystem.
func purgeOldPromptInputFiles(base string, retention int) {
	if retention <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retention).Format("2006-01-02")
	purgeOldPromptInputFilesInDir(base, cutoff, true)
	entries, err := os.ReadDir(base)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		purgeOldPromptInputFilesInDir(filepath.Join(base, entry.Name()), cutoff, false)
	}
}

// purgeOldPromptInputFilesInDir deletes expired daily files in dir. When
// includeFlat is true it also matches the legacy prompt-input-*.log files.
func purgeOldPromptInputFilesInDir(dir, cutoff string, includeFlat bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		date := ""
		if len(name) == len("2006-01-02.log") {
			date = strings.TrimSuffix(name, ".log")
		} else if includeFlat && strings.HasPrefix(name, promptInputFilenamePrefix) {
			date = strings.TrimSuffix(strings.TrimPrefix(name, promptInputFilenamePrefix), ".log")
		} else {
			continue
		}
		if len(date) != len("2006-01-02") || date >= cutoff {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
