package pipeline

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/huangke/bt-spider/pkg/logger"
	"github.com/huangke/bt-spider/search"
	_ "modernc.org/sqlite"
)

var (
	auditMu      sync.Mutex
	auditDB      *sql.DB
	auditPath    string
	auditInitErr error
)

const auditSchema = `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS search_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	keyword TEXT NOT NULL,
	strict_mode INTEGER NOT NULL DEFAULT 0,
	timeout_ms INTEGER NOT NULL,
	provider_total INTEGER NOT NULL,
	started_at TEXT NOT NULL,
	finished_at TEXT,
	status TEXT NOT NULL DEFAULT 'running',
	final_result_count INTEGER NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS provider_attempts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id INTEGER NOT NULL,
	provider_name TEXT NOT NULL,
	status TEXT NOT NULL,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	result_count INTEGER NOT NULL DEFAULT 0,
	error_message TEXT NOT NULL DEFAULT '',
	recorded_at TEXT NOT NULL,
	FOREIGN KEY(run_id) REFERENCES search_runs(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS provider_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	run_id INTEGER NOT NULL,
	provider_attempt_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	size TEXT NOT NULL,
	seeders INTEGER NOT NULL,
	leechers INTEGER NOT NULL,
	magnet TEXT NOT NULL,
	source TEXT NOT NULL,
	info_hash TEXT NOT NULL,
	recorded_at TEXT NOT NULL,
	FOREIGN KEY(run_id) REFERENCES search_runs(id) ON DELETE CASCADE,
	FOREIGN KEY(provider_attempt_id) REFERENCES provider_attempts(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_search_runs_started_at ON search_runs(started_at);
CREATE INDEX IF NOT EXISTS idx_provider_attempts_run_id ON provider_attempts(run_id);
CREATE INDEX IF NOT EXISTS idx_provider_items_run_id ON provider_items(run_id);
CREATE INDEX IF NOT EXISTS idx_provider_items_info_hash ON provider_items(info_hash);
`

// SetSearchAuditDBPath 设置搜索审计数据库路径。
// path 为空时使用默认路径：~/Library/Application Support/BT-Spider/search_history.db。
func SetSearchAuditDBPath(path string) error {
	auditMu.Lock()
	defer auditMu.Unlock()

	resolved, err := resolveAuditDBPath(path)
	if err != nil {
		return err
	}

	if auditPath == resolved && auditDB != nil {
		return nil
	}

	if auditDB != nil {
		_ = auditDB.Close()
		auditDB = nil
	}
	auditPath = resolved
	auditInitErr = nil

	db, err := openAuditDBLocked()
	if err != nil {
		auditInitErr = err
		return err
	}
	auditDB = db
	return nil
}

func resolveAuditDBPath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("获取 HOME 失败: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "BT-Spider", "search_history.db"), nil
	}

	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("获取 HOME 失败: %w", err)
		}
		trimmed = filepath.Join(home, trimmed[2:])
	}

	return trimmed, nil
}

func ensureAuditDB() (*sql.DB, error) {
	auditMu.Lock()
	defer auditMu.Unlock()

	if auditDB != nil {
		return auditDB, nil
	}
	if auditInitErr != nil {
		return nil, auditInitErr
	}

	db, err := openAuditDBLocked()
	if err != nil {
		auditInitErr = err
		return nil, err
	}
	auditDB = db
	return auditDB, nil
}

func openAuditDBLocked() (*sql.DB, error) {
	if strings.TrimSpace(auditPath) == "" {
		resolved, err := resolveAuditDBPath("")
		if err != nil {
			return nil, err
		}
		auditPath = resolved
	}

	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		return nil, fmt.Errorf("创建审计数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", auditPath)
	if err != nil {
		return nil, fmt.Errorf("打开审计数据库失败: %w", err)
	}

	if _, err := db.Exec(auditSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("初始化审计数据库失败: %w", err)
	}

	return db, nil
}

func auditStartRun(keyword string, timeout time.Duration, strictMode bool, providerTotal int) int64 {
	db, err := ensureAuditDB()
	if err != nil {
		logger.Warn("audit disabled", "err", err)
		return 0
	}

	now := time.Now().Format(time.RFC3339Nano)
	strictModeInt := 0
	if strictMode {
		strictModeInt = 1
	}

	res, err := db.Exec(
		`INSERT INTO search_runs (keyword, strict_mode, timeout_ms, provider_total, started_at, status)
		 VALUES (?, ?, ?, ?, ?, 'running')`,
		keyword,
		strictModeInt,
		timeout.Milliseconds(),
		providerTotal,
		now,
	)
	if err != nil {
		logger.Warn("audit start run failed", "keyword", keyword, "err", err)
		return 0
	}

	runID, err := res.LastInsertId()
	if err != nil {
		logger.Warn("audit start run no id", "keyword", keyword, "err", err)
		return 0
	}
	return runID
}

func auditRecordProviderResultSync(runID int64, provider string, duration time.Duration, results []search.Result, err error) {
	if runID == 0 {
		return
	}

	db, openErr := ensureAuditDB()
	if openErr != nil {
		return
	}

	status := "success"
	errMsg := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
		results = nil
	}

	recordedAt := time.Now().Format(time.RFC3339Nano)
	tx, beginErr := db.Begin()
	if beginErr != nil {
		logger.Warn("audit begin tx failed", "provider", provider, "err", beginErr)
		return
	}

	res, execErr := tx.Exec(
		`INSERT INTO provider_attempts (run_id, provider_name, status, duration_ms, result_count, error_message, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		runID,
		provider,
		status,
		duration.Milliseconds(),
		len(results),
		errMsg,
		recordedAt,
	)
	if execErr != nil {
		_ = tx.Rollback()
		logger.Warn("audit provider attempt insert failed", "provider", provider, "err", execErr)
		return
	}

	providerAttemptID, idErr := res.LastInsertId()
	if idErr != nil {
		_ = tx.Rollback()
		logger.Warn("audit provider attempt no id", "provider", provider, "err", idErr)
		return
	}

	if len(results) > 0 {
		stmt, prepErr := tx.Prepare(
			`INSERT INTO provider_items
			 (run_id, provider_attempt_id, name, size, seeders, leechers, magnet, source, info_hash, recorded_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		)
		if prepErr != nil {
			_ = tx.Rollback()
			logger.Warn("audit provider items prepare failed", "provider", provider, "err", prepErr)
			return
		}
		defer stmt.Close()

		for _, r := range results {
			if _, rowErr := stmt.Exec(
				runID,
				providerAttemptID,
				r.Name,
				r.Size,
				r.Seeders,
				r.Leechers,
				r.Magnet,
				r.Source,
				r.InfoHash,
				recordedAt,
			); rowErr != nil {
				_ = tx.Rollback()
				logger.Warn("audit provider item insert failed", "provider", provider, "err", rowErr)
				return
			}
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		logger.Warn("audit commit failed", "provider", provider, "err", commitErr)
	}
}

func auditRecordProviderTimeoutSync(runID int64, provider string, timeout time.Duration) {
	if runID == 0 {
		return
	}

	db, err := ensureAuditDB()
	if err != nil {
		return
	}

	_, execErr := db.Exec(
		`INSERT INTO provider_attempts (run_id, provider_name, status, duration_ms, result_count, error_message, recorded_at)
		 VALUES (?, ?, 'timeout', ?, 0, ?, ?)`,
		runID,
		provider,
		timeout.Milliseconds(),
		"搜索超时，provider 未在截止前返回",
		time.Now().Format(time.RFC3339Nano),
	)
	if execErr != nil {
		logger.Warn("audit provider timeout insert failed", "provider", provider, "err", execErr)
	}
}

func auditFinishRunSync(runID int64, status string, finalResultCount int, errMsg string) {
	if runID == 0 {
		return
	}

	db, err := ensureAuditDB()
	if err != nil {
		return
	}

	if strings.TrimSpace(status) == "" {
		status = "unknown"
	}
	if errMsg == "" {
		errMsg = ""
	}

	_, execErr := db.Exec(
		`UPDATE search_runs
		 SET finished_at = ?, status = ?, final_result_count = ?, error_message = ?
		 WHERE id = ?`,
		time.Now().Format(time.RFC3339Nano),
		status,
		finalResultCount,
		errMsg,
		runID,
	)
	if execErr != nil {
		logger.Warn("audit finish run failed", "run_id", runID, "err", execErr)
	}
}

func isAuditDBConfigured() bool {
	auditMu.Lock()
	defer auditMu.Unlock()
	return strings.TrimSpace(auditPath) != ""
}

func closeAuditDB() error {
	if auditQueue != nil {
		close(auditQueue)
		auditWorkerWG.Wait()
	}

	auditMu.Lock()
	defer auditMu.Unlock()
	if auditDB == nil {
		return nil
	}
	err := auditDB.Close()
	auditDB = nil
	if err != nil {
		return fmt.Errorf("关闭审计数据库失败: %w", err)
	}
	return nil
}

func auditHealthcheck() error {
	db, err := ensureAuditDB()
	if err != nil {
		return err
	}
	if db == nil {
		return errors.New("审计数据库未初始化")
	}
	return db.Ping()
}

// --- 异步写入队列 ---

type auditJob struct {
	kind string // "provider_result" | "provider_timeout" | "finish_run"

	// provider_result / provider_timeout 共用
	runID    int64
	provider string
	duration time.Duration

	// provider_result 专用
	results []search.Result
	runErr  error

	// finish_run 专用
	status     string
	finalCount int
	errMsg     string
}

var (
	auditQueue     chan auditJob
	auditQueueOnce sync.Once
	auditWorkerWG  sync.WaitGroup
)

const auditQueueCap = 1024

func initAuditQueue() {
	auditQueueOnce.Do(func() {
		auditQueue = make(chan auditJob, auditQueueCap)
		auditWorkerWG.Add(1)
		go runAuditWorker()
	})
}

func runAuditWorker() {
	defer auditWorkerWG.Done()
	for job := range auditQueue {
		switch job.kind {
		case "provider_result":
			auditRecordProviderResultSync(job.runID, job.provider, job.duration, job.results, job.runErr)
		case "provider_timeout":
			auditRecordProviderTimeoutSync(job.runID, job.provider, job.duration)
		case "finish_run":
			auditFinishRunSync(job.runID, job.status, job.finalCount, job.errMsg)
		}
	}
}

func auditRecordProviderResult(runID int64, provider string, duration time.Duration, results []search.Result, err error) {
	if runID == 0 {
		return
	}
	initAuditQueue()
	// 深拷贝 results，避免生产者后续修改导致 race
	cp := append([]search.Result(nil), results...)
	job := auditJob{
		kind: "provider_result", runID: runID, provider: provider,
		duration: duration, results: cp, runErr: err,
	}
	select {
	case auditQueue <- job:
	default:
		logger.Warn("audit queue full, dropping", "kind", "provider_result", "provider", provider)
	}
}

func auditRecordProviderTimeout(runID int64, provider string, timeout time.Duration) {
	if runID == 0 {
		return
	}
	initAuditQueue()
	job := auditJob{kind: "provider_timeout", runID: runID, provider: provider, duration: timeout}
	select {
	case auditQueue <- job:
	default:
		logger.Warn("audit queue full, dropping", "kind", "provider_timeout", "provider", provider)
	}
}

func auditFinishRun(runID int64, status string, finalResultCount int, errMsg string) {
	if runID == 0 {
		return
	}
	initAuditQueue()
	job := auditJob{kind: "finish_run", runID: runID, status: status, finalCount: finalResultCount, errMsg: errMsg}
	select {
	case auditQueue <- job:
	default:
		logger.Warn("audit queue full, dropping", "kind", "finish_run", "run_id", runID)
	}
}
