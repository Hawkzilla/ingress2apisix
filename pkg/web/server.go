package web

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingress2apisix/pkg/apisix"
	"github.com/ingress2apisix/pkg/charts"
	"github.com/ingress2apisix/pkg/converter"
)

// Server serves the ingress2apisix web UI and API.
type Server struct {
	mux          *http.ServeMux
	opts         apisix.ConversionOptions
	addr         string
	version      string
	feedback     *FeedbackStore
	announcement *AnnouncementStore
}

// NewServer creates a new web server.
func NewServer(addr string, opts apisix.ConversionOptions, ver string) *Server {
	s := &Server{
		mux:     http.NewServeMux(),
		opts:    opts,
		addr:    addr,
		version: ver,
	}

	// Initialize feedback store (non-fatal on failure)
	if store, err := NewFeedbackStore("feedback.db"); err != nil {
		fmt.Printf("Warning: failed to open feedback database: %v\n", err)
	} else {
		s.feedback = store
	}

	// Initialize announcement store
	if store, err := NewAnnouncementStore("announcements.json"); err != nil {
		fmt.Printf("Warning: failed to open announcements file: %v\n", err)
	} else {
		s.announcement = store
	}

	s.routes()
	return s
}

// routes registers all HTTP handlers.
func (s *Server) routes() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/api/convert", s.handleConvert)
	s.mux.HandleFunc("/api/check", s.handleCheck)
	s.mux.HandleFunc("/api/migrate", s.handleMigrate)
	s.mux.HandleFunc("/api/download/migrate-tar", s.handleDownloadMigrateTar)
	s.mux.HandleFunc("/api/docs/migration", s.handleDocMigration)
	s.mux.HandleFunc("/api/docs/annotations", s.handleDocAnnotations)
	s.mux.HandleFunc("/api/docs/usage", s.handleDocUsage)
	s.mux.HandleFunc("/api/docs/webui", s.handleDocWebUI)
	s.mux.HandleFunc("/api/docs/multi-region-idp-proxy", s.handleDocMultiRegionIDP)
	s.mux.HandleFunc("/api/feedback", s.handleFeedback)
	s.mux.HandleFunc("/api/announcements", s.handleAnnouncements)
	s.RegisterUploadRoutes()
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	fmt.Printf("Starting ingress2apisix web UI at http://%s\n", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

// handleIndex serves the web UI and static assets.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		// Serve static files
		if strings.HasPrefix(r.URL.Path, "/static/") {
			name := strings.TrimPrefix(r.URL.Path, "/")
			data, err := staticFS.ReadFile(name)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			ct := "text/plain"
			switch {
			case strings.HasSuffix(name, ".png"):
				ct = "image/png"
			case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
				ct = "image/jpeg"
			case strings.HasSuffix(name, ".gif"):
				ct = "image/gif"
			case strings.HasSuffix(name, ".svg"):
				ct = "image/svg+xml"
			case strings.HasSuffix(name, ".css"):
				ct = "text/css"
			case strings.HasSuffix(name, ".js"):
				ct = "application/javascript"
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Cache-Control", "max-age=3600")
			w.Write(data)
			return
		}
		http.NotFound(w, r)
		return
	}

	// Serve index.html with version injection
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
		return
	}
	html := strings.ReplaceAll(string(data), "{{VERSION}}", s.version)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// --- Request / Response types ---

type convertRequest struct {
	YAML        string `json:"yaml"`
	SslRedirect bool   `json:"sslRedirect"`
}

type convertResponse struct {
	Success  bool     `json:"success"`
	Output   string   `json:"output"`
	Summary  string   `json:"summary"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}

type checkRequest struct {
	YAML string `json:"yaml"`
}

type checkResponse struct {
	Success      bool              `json:"success"`
	Converted    int               `json:"converted"`
	PluginConfig int               `json:"pluginConfig"`
	CustomPlugin int               `json:"customPlugin"`
	Manual       int               `json:"manual"`
	Unknown      int               `json:"unknown"`
	TotalFiles   int               `json:"totalFiles"`
	IngressFiles int               `json:"ingressFiles"`
	HasManual    bool              `json:"hasManual"`
	HasUnknown   bool              `json:"hasUnknown"`
	Warnings     []string          `json:"warnings"`
	Files        []checkFileResult `json:"files"`
}

type checkFileResult struct {
	Path       string         `json:"path"`
	HasHelmTpl bool           `json:"hasHelmTpl"`
	Findings   []checkFinding `json:"findings"`
}

type checkFinding struct {
	Annotation string `json:"annotation"`
	Status     string `json:"status"`
	Detail     string `json:"detail"`
}

type migrateRequest struct {
	YAML string `json:"yaml"`
}

type migrateResponse struct {
	Success  bool     `json:"success"`
	Output   string   `json:"output"`
	Report   string   `json:"report"`
	Warnings []string `json:"warnings"`
}

// --- API handlers ---

func (s *Server) handleConvert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req convertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, convertResponse{
			Success: false,
			Errors:  []string{"Invalid JSON: " + err.Error()},
		})
		return
	}

	parsed, err := converter.ParseIngressYAML([]byte(req.YAML))
	if err != nil {
		writeJSON(w, http.StatusOK, convertResponse{
			Success: false,
			Errors:  []string{"Failed to parse YAML: " + err.Error()},
		})
		return
	}

	opts := s.opts
	opts.SSLRedirect = req.SslRedirect
	c := converter.New(opts)

	var result apisix.ConversionResult
	result.InputFormat = parsed.Format
	for _, ing := range parsed.Ingresses {
		r := c.Convert(ing)
		result.Ingresses = append(result.Ingresses, r.Ingresses...)
		result.PluginConfigs = append(result.PluginConfigs, r.PluginConfigs...)
		result.BackendTrafficPolicies = append(result.BackendTrafficPolicies, r.BackendTrafficPolicies...)
		result.Warnings = append(result.Warnings, r.Warnings...)
		result.Errors = append(result.Errors, r.Errors...)
	}

	var buf bytes.Buffer
	if err := converter.WriteConversionResult(&buf, result); err != nil {
		writeJSON(w, http.StatusOK, convertResponse{
			Success: false,
			Errors:  []string{"Failed to write output: " + err.Error()},
		})
		return
	}

	summary := converter.FormatResultSummary(result)
	errStrs := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errStrs[i] = e.Error()
	}

	writeJSON(w, http.StatusOK, convertResponse{
		Success:  true,
		Output:   buf.String(),
		Summary:  summary,
		Warnings: result.Warnings,
		Errors:   errStrs,
	})
}

func (s *Server) handleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req checkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, checkResponse{
			Success:  false,
			Warnings: []string{"Invalid JSON: " + err.Error()},
		})
		return
	}

	// Write YAML to a temp directory so CheckChartsDir can scan it
	tmpDir, err := os.MkdirTemp("", "ingress2apisix-check-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, checkResponse{
			Success:  false,
			Warnings: []string{"Failed to create temp dir: " + err.Error()},
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	if err := os.MkdirAll(filepath.Join(tmpDir, "templates"), 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, checkResponse{
			Success:  false,
			Warnings: []string{"Failed to create templates dir: " + err.Error()},
		})
		return
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "templates", "ingress.yaml"), []byte(req.YAML), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, checkResponse{
			Success:  false,
			Warnings: []string{"Failed to write YAML: " + err.Error()},
		})
		return
	}

	report, err := charts.CheckChartsDir(tmpDir)
	if err != nil {
		writeJSON(w, http.StatusOK, checkResponse{
			Success:  false,
			Warnings: []string{"Check failed: " + err.Error()},
		})
		return
	}

	var files []checkFileResult
	for _, f := range report.Files {
		cf := checkFileResult{
			Path:       f.Path,
			HasHelmTpl: f.HasHelmTpl,
		}
		for _, finding := range f.Findings {
			cf.Findings = append(cf.Findings, checkFinding{
				Annotation: finding.Annotation,
				Status:     finding.Status.String(),
				Detail:     finding.Detail,
			})
		}
		files = append(files, cf)
	}

	writeJSON(w, http.StatusOK, checkResponse{
		Success:      true,
		Converted:    report.Converted,
		PluginConfig: report.PluginConfig,
		CustomPlugin: report.CustomPlugin,
		Manual:       report.Manual,
		Unknown:      report.Unknown,
		TotalFiles:   report.TotalFiles,
		IngressFiles: report.IngressFiles,
		HasManual:    report.Manual > 0,
		HasUnknown:   report.Unknown > 0,
		Warnings:     []string{},
		Files:        files,
	})
}

func (s *Server) handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req migrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, migrateResponse{
			Success: false,
			Report:  "Invalid JSON: " + err.Error(),
		})
		return
	}

	tmpDir, err := os.MkdirTemp("", "ingress2apisix-migrate-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, migrateResponse{
			Success: false,
			Report:  "Failed to create temp dir: " + err.Error(),
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, migrateResponse{
			Success: false,
			Report:  "Failed to create templates dir: " + err.Error(),
		})
		return
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(req.YAML), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, migrateResponse{
			Success: false,
			Report:  "Failed to write YAML: " + err.Error(),
		})
		return
	}

	report, err := charts.MigrateChartsDir(tmpDir, charts.MigrateOptions{DryRun: false})
	if err != nil {
		writeJSON(w, http.StatusOK, migrateResponse{
			Success: false,
			Report:  "Migration failed: " + err.Error(),
		})
		return
	}

	// Read back modified files
	var output strings.Builder
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		fmt.Fprintf(&output, "--- # %s\n%s\n", relPath, string(data))
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusOK, migrateResponse{
			Success: false,
			Report:  "Failed to read results: " + err.Error(),
		})
		return
	}

	reportText := fmt.Sprintf("Files processed: %d\nFiles modified: %d\nPluginConfigs generated: %d",
		report.FilesProcessed, report.FilesModified, report.PluginConfigs)

	writeJSON(w, http.StatusOK, migrateResponse{
		Success:  true,
		Output:   output.String(),
		Report:   reportText,
		Warnings: report.Warnings,
	})
}

func (s *Server) handleDownloadMigrateTar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req migrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	tmpDir, err := os.MkdirTemp("", "ingress2apisix-tar-*")
	if err != nil {
		http.Error(w, "Failed to create temp dir", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tmpDir)

	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		http.Error(w, "Failed to create templates dir", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "ingress.yaml"), []byte(req.YAML), 0644); err != nil {
		http.Error(w, "Failed to write YAML", http.StatusInternalServerError)
		return
	}

	_, err = charts.MigrateChartsDir(tmpDir, charts.MigrateOptions{DryRun: false})
	if err != nil {
		http.Error(w, "Migration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect files for tar.gz
	type tarEntry struct {
		Name string
		Data []byte
	}
	var entries []tarEntry

	chartYaml := []byte("apiVersion: v2\nname: migrated\nversion: 0.1.0\ndescription: Migrated by ingress2apisix\n")
	entries = append(entries, tarEntry{"Chart.yaml", chartYaml})

	valuesYaml := []byte("# Migrated by ingress2apisix\n")
	entries = append(entries, tarEntry{"values.yaml", valuesYaml})

	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		entries = append(entries, tarEntry{relPath, data})
		return nil
	})
	if err != nil {
		http.Error(w, "Failed to collect files", http.StatusInternalServerError)
		return
	}

	// Build tar.gz
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, entry := range entries {
		hdr := &tar.Header{
			Name: entry.Name,
			Mode: 0644,
			Size: int64(len(entry.Data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			http.Error(w, "Failed to write tar header", http.StatusInternalServerError)
			return
		}
		if _, err := tw.Write(entry.Data); err != nil {
			http.Error(w, "Failed to write tar data", http.StatusInternalServerError)
			return
		}
	}

	tw.Close()
	gz.Close()

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="migrated-charts.tar.gz"`)
	w.Write(buf.Bytes())
}

// --- Doc handlers ---

func (s *Server) handleDocMigration(w http.ResponseWriter, r *http.Request) {
	serveDoc(w, "migration.md")
}

func (s *Server) handleDocAnnotations(w http.ResponseWriter, r *http.Request) {
	serveDoc(w, "annotations.md")
}

func (s *Server) handleDocUsage(w http.ResponseWriter, r *http.Request) {
	serveDoc(w, "usage.md")
}

func (s *Server) handleDocWebUI(w http.ResponseWriter, r *http.Request) {
	serveDoc(w, "webui.md")
}

func (s *Server) handleDocMultiRegionIDP(w http.ResponseWriter, r *http.Request) {
	serveDoc(w, "multi-region-idp-proxy.md")
}

func serveDoc(w http.ResponseWriter, name string) {
	data, err := docsFS.ReadFile("docs/" + name)
	if err != nil {
		http.Error(w, "Document not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

// --- Feedback handlers ---

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	if s.feedback == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Feedback database not available",
		})
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.handleFeedbackCreate(w, r)
	case http.MethodGet:
		s.handleFeedbackList(w, r)
	case http.MethodDelete:
		s.handleFeedbackDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleFeedbackCreate(w http.ResponseWriter, r *http.Request) {
	var fb Feedback
	if err := json.NewDecoder(r.Body).Decode(&fb); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid JSON: " + err.Error(),
		})
		return
	}

	if fb.Title == "" || fb.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Title and content are required",
		})
		return
	}

	validCategories := map[string]bool{
		"tool": true, "doc": true, "feature": true, "general": true,
	}
	if !validCategories[fb.Category] {
		fb.Category = "general"
	}

	if err := s.feedback.Add(&fb); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to save feedback: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    fb,
	})
}

func (s *Server) handleFeedbackList(w http.ResponseWriter, r *http.Request) {
	page, pageSize := 1, 20
	if p := r.URL.Query().Get("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize
	items, total, err := s.feedback.List(pageSize, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to list feedback: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"data":     items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *Server) handleFeedbackDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Missing id parameter",
		})
		return
	}

	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "Invalid id parameter",
		})
		return
	}

	if err := s.feedback.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Failed to delete feedback: " + err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// --- Announcement handlers ---

func (s *Server) handleAnnouncements(w http.ResponseWriter, r *http.Request) {
	if s.announcement == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"error":   "Announcement store not available",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleAnnouncementList(w, r)
	case http.MethodPost:
		s.handleAnnouncementCreate(w, r)
	case http.MethodPut:
		s.handleAnnouncementUpdate(w, r)
	case http.MethodDelete:
		s.handleAnnouncementDelete(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleAnnouncementList(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("all") != "true"
	data := s.announcement.List(activeOnly)
	if data == nil {
		data = []Announcement{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    data,
	})
}

func (s *Server) handleAnnouncementCreate(w http.ResponseWriter, r *http.Request) {
	var a Announcement
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Invalid JSON"})
		return
	}
	if a.Title == "" || a.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Title and content required"})
		return
	}
	a.Active = true
	if err := s.announcement.Add(&a); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "data": a})
}

func (s *Server) handleAnnouncementUpdate(w http.ResponseWriter, r *http.Request) {
	var a Announcement
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Invalid JSON"})
		return
	}
	if a.ID == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Missing id"})
		return
	}
	if err := s.announcement.Update(&a); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (s *Server) handleAnnouncementDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	var id int64
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil || id == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "Invalid id"})
		return
	}
	if err := s.announcement.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

// --- Upload handlers ---

// RegisterUploadRoutes registers multipart file upload endpoints.
func (s *Server) RegisterUploadRoutes() {
	s.mux.HandleFunc("/api/upload/check", s.handleUploadCheck)
	s.mux.HandleFunc("/api/upload/migrate", s.handleUploadMigrate)
}

func (s *Server) handleUploadCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, checkResponse{
			Success:  false,
			Warnings: []string{"Failed to parse upload: " + err.Error()},
		})
		return
	}

	tmpDir, err := os.MkdirTemp("", "ingress2apisix-upload-check-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, checkResponse{
			Success:  false,
			Warnings: []string{"Failed to create temp dir: " + err.Error()},
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, checkResponse{
			Success:  false,
			Warnings: []string{"No files uploaded"},
		})
		return
	}

	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			continue
		}
		dstPath := filepath.Join(tmpDir, fh.Filename)
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		os.WriteFile(dstPath, data, 0644)
	}

	report, err := charts.CheckChartsDir(tmpDir)
	if err != nil {
		writeJSON(w, http.StatusOK, checkResponse{
			Success:  false,
			Warnings: []string{"Check failed: " + err.Error()},
		})
		return
	}

	var fileResults []checkFileResult
	for _, f := range report.Files {
		cf := checkFileResult{
			Path:       f.Path,
			HasHelmTpl: f.HasHelmTpl,
		}
		for _, finding := range f.Findings {
			cf.Findings = append(cf.Findings, checkFinding{
				Annotation: finding.Annotation,
				Status:     finding.Status.String(),
				Detail:     finding.Detail,
			})
		}
		fileResults = append(fileResults, cf)
	}

	writeJSON(w, http.StatusOK, checkResponse{
		Success:      true,
		Converted:    report.Converted,
		PluginConfig: report.PluginConfig,
		CustomPlugin: report.CustomPlugin,
		Manual:       report.Manual,
		Unknown:      report.Unknown,
		TotalFiles:   report.TotalFiles,
		IngressFiles: report.IngressFiles,
		HasManual:    report.Manual > 0,
		HasUnknown:   report.Unknown > 0,
		Warnings:     []string{},
		Files:        fileResults,
	})
}

func (s *Server) handleUploadMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(64 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, migrateResponse{
			Success: false,
			Report:  "Failed to parse upload: " + err.Error(),
		})
		return
	}

	tmpDir, err := os.MkdirTemp("", "ingress2apisix-upload-migrate-*")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, migrateResponse{
			Success: false,
			Report:  "Failed to create temp dir: " + err.Error(),
		})
		return
	}
	defer os.RemoveAll(tmpDir)

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeJSON(w, http.StatusBadRequest, migrateResponse{
			Success: false,
			Report:  "No files uploaded",
		})
		return
	}

	for _, fh := range files {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			continue
		}
		dstPath := filepath.Join(tmpDir, fh.Filename)
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		os.WriteFile(dstPath, data, 0644)
	}

	report, err := charts.MigrateChartsDir(tmpDir, charts.MigrateOptions{DryRun: false})
	if err != nil {
		writeJSON(w, http.StatusOK, migrateResponse{
			Success: false,
			Report:  "Migration failed: " + err.Error(),
		})
		return
	}

	var output strings.Builder
	err = filepath.Walk(tmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		relPath, _ := filepath.Rel(tmpDir, path)
		fmt.Fprintf(&output, "--- # %s\n%s\n", relPath, string(data))
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusOK, migrateResponse{
			Success: false,
			Report:  "Failed to read results: " + err.Error(),
		})
		return
	}

	reportText := fmt.Sprintf("Files processed: %d\nFiles modified: %d\nPluginConfigs generated: %d",
		report.FilesProcessed, report.FilesModified, report.PluginConfigs)

	writeJSON(w, http.StatusOK, migrateResponse{
		Success:  true,
		Output:   output.String(),
		Report:   reportText,
		Warnings: report.Warnings,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
