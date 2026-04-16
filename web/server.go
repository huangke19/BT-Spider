package web

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/huangke/bt-spider/engine"
	"github.com/huangke/bt-spider/pkg/utils"
	"github.com/huangke/bt-spider/search"
)

//go:embed static/index.html
var indexHTML string

type Server struct {
	engine *engine.Engine
}

func New(eng *engine.Engine) *Server {
	return &Server{engine: eng}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/tasks", s.handleTasks)
	mux.HandleFunc("/api/tasks/", s.handleTaskByID)
	mux.HandleFunc("/api/tasks/clear", s.handleClearFinished)
	return withLog(mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"download_dir":     s.engine.Config().DownloadDir,
		"max_results":      s.engine.Config().MaxResults,
		"seed":             s.engine.Config().Seed,
		"seed_ratio_limit": s.engine.Config().SeedRatioLimit,
		"seed_time_limit":  s.engine.Config().SeedTimeLimit,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "请输入搜索关键词")
		return
	}

	results, err := search.Search(req.Query, search.DefaultProviders())
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	limit := s.engine.Config().MaxResults
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"query":   req.Query,
		"results": results,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	var req struct {
		Magnet         string   `json:"magnet"`
		Name           string   `json:"name"`
		Seed           *bool    `json:"seed"`
		SeedRatioLimit *float64 `json:"seed_ratio_limit"`
		SeedTimeLimit  *string  `json:"seed_time_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	req.Magnet = strings.TrimSpace(req.Magnet)
	if req.Magnet == "" {
		writeError(w, http.StatusBadRequest, "磁力链接不能为空")
		return
	}

	policy, err := s.downloadPolicyFromRequest(req.Seed, req.SeedRatioLimit, req.SeedTimeLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	dl, err := s.engine.AddMagnetWithPolicyAsync(req.Magnet, policy)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	snap := dl.Snapshot()
	writeJSON(w, http.StatusCreated, map[string]any{
		"message": fmt.Sprintf("已加入下载队列: %s", displayName(req.Name, snap.Name)),
		"task":    toTaskDTO(snap),
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}

	snaps := s.engine.ListDownloads()
	tasks := make([]taskDTO, 0, len(snaps))
	for _, snap := range snaps {
		tasks = append(tasks, toTaskDTO(snap))
	}

	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks})
}

func (s *Server) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		methodNotAllowed(w, http.MethodDelete)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, http.StatusBadRequest, "任务 ID 不能为空")
		return
	}

	if !s.engine.RemoveDownload(id) {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"message": "任务已取消"})
}

func (s *Server) handleClearFinished(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}

	removed := s.engine.ClearFinished()
	writeJSON(w, http.StatusOK, map[string]any{
		"message": fmt.Sprintf("已清理 %d 个已结束任务", removed),
		"removed": removed,
	})
}

type taskDTO struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	Completed     int64   `json:"completed"`
	CompletedText string  `json:"completed_text"`
	Total         int64   `json:"total"`
	TotalText     string  `json:"total_text"`
	Percent       float64 `json:"percent"`
	SpeedText     string  `json:"speed_text"`
	Peers         int     `json:"peers"`
	ETAText       string  `json:"eta_text"`
	UploadedText  string  `json:"uploaded_text"`
	ShareRatio    float64 `json:"share_ratio"`
	SeedElapsed   string  `json:"seed_elapsed"`
	Error         string  `json:"error"`
}

func toTaskDTO(snap engine.DownloadSnapshot) taskDTO {
	percent := 0.0
	if snap.Total > 0 {
		percent = float64(snap.Completed) / float64(snap.Total) * 100
		if percent > 100 {
			percent = 100
		}
	}

	eta := "-"
	if snap.ETA > 0 {
		eta = utils.FormatDuration(snap.ETA)
	}
	if snap.State == engine.StateDone {
		eta = "完成"
	}
	seedElapsed := "-"
	if snap.SeedElapsed > 0 {
		seedElapsed = utils.FormatDuration(snap.SeedElapsed)
	}

	return taskDTO{
		ID:            snap.ID,
		Name:          snap.Name,
		State:         snap.State.String(),
		Completed:     snap.Completed,
		CompletedText: utils.FormatBytes(snap.Completed),
		Total:         snap.Total,
		TotalText:     utils.FormatBytes(snap.Total),
		Percent:       percent,
		SpeedText:     utils.FormatBytes(int64(snap.Speed)) + "/s",
		Peers:         snap.Peers,
		ETAText:       eta,
		UploadedText:  utils.FormatBytes(snap.Uploaded),
		ShareRatio:    snap.ShareRatio,
		SeedElapsed:   seedElapsed,
		Error:         snap.Err,
	}
}

func displayName(preferred, fallback string) string {
	preferred = strings.TrimSpace(preferred)
	if preferred != "" {
		return preferred
	}
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	return "新任务"
}

func (s *Server) downloadPolicyFromRequest(seed *bool, ratio *float64, timeLimit *string) (engine.DownloadPolicy, error) {
	cfg := s.engine.Config()
	policy := engine.DownloadPolicy{
		Seed:           cfg.Seed,
		SeedRatioLimit: cfg.SeedRatioLimit,
	}
	if d, err := cfg.SeedTimeLimitDuration(); err == nil {
		policy.SeedTimeLimit = d
	}

	if seed != nil {
		policy.Seed = *seed
	}
	if ratio != nil {
		if *ratio < 0 {
			return engine.DownloadPolicy{}, fmt.Errorf("分享率上限不能小于 0")
		}
		policy.SeedRatioLimit = *ratio
	}
	if timeLimit != nil {
		trimmed := strings.TrimSpace(*timeLimit)
		if trimmed == "" {
			trimmed = "0s"
		}
		d, err := time.ParseDuration(trimmed)
		if err != nil {
			return engine.DownloadPolicy{}, fmt.Errorf("保种时长格式不正确")
		}
		if d < 0 {
			return engine.DownloadPolicy{}, fmt.Errorf("保种时长不能小于 0")
		}
		policy.SeedTimeLimit = d
	}
	return policy, nil
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
