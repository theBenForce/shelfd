package plugins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shelfd/shelfd/internal/events"
	"github.com/shelfd/shelfd/internal/jobs"
	"github.com/shelfd/shelfd/internal/repository"
)

const (
	maxEventPayloadBytes      = 64 << 10 // 64 KB cap per plugin notification
	maxRPCFrameBytes          = 10 << 20 // 10 MB max NDJSON line from plugin stdout
	maxLogLines               = 100      // Ring buffer size per plugin
	handshakeTimeout          = 30 * time.Second
	shutdownGracePeriod       = 5 * time.Second
	defaultCallTimeout        = 60 * time.Second
	stageCallTimeout          = 5 * time.Minute
	defaultJobWatchdogTimeout = 5 * time.Minute
)

var (
	ErrPluginNotFound = errors.New("plugin not found")
	ErrPluginDisabled = errors.New("plugin is disabled")
)

// TokenIssuer mints a short-lived, user-scoped and scope-restricted JWT for loopback API calls.
type TokenIssuer func(userID, username, pluginName string, scopes []string) (string, error)

// HostConfig configures the Shelfd Plugin Host.
type HostConfig struct {
	PluginsDir     string
	APIBase        string
	ShelfdVersion  string
	MasterSecret   string
	IdleTimeout    time.Duration
	JobIdleTimeout time.Duration
	TokenIssuer    TokenIssuer
	Hub            *events.Hub
	JobManager     *jobs.Manager
	Logger         *slog.Logger
}

type pluginProcess struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	writeMu  sync.Mutex
	pending  map[string]chan RPCMessage
	pendMu   sync.Mutex
	doneCh   chan struct{}
	nextID   atomic.Uint64
	stopping atomic.Bool
}

func (p *pluginProcess) sendJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if _, err := p.stdin.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func (p *pluginProcess) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	idStr := strconv.FormatUint(p.nextID.Add(1), 10)
	idRaw, _ := json.Marshal(idStr)
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshaling params for %s: %w", method, err)
	}

	respCh := make(chan RPCMessage, 1)
	p.pendMu.Lock()
	p.pending[idStr] = respCh
	p.pendMu.Unlock()

	defer func() {
		p.pendMu.Lock()
		delete(p.pending, idStr)
		p.pendMu.Unlock()
	}()

	req := RPCMessage{
		JSONRPC: "2.0",
		ID:      idRaw,
		Method:  method,
		Params:  paramsRaw,
	}
	if err := p.sendJSON(req); err != nil {
		return nil, fmt.Errorf("writing %s request to plugin stdin: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.doneCh:
		return nil, errors.New("plugin process exited before responding")
	case resp := <-respCh:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (p *pluginProcess) notify(method string, params any) error {
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return p.sendJSON(RPCMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsRaw,
	})
}

type managedPlugin struct {
	mu                  sync.Mutex
	dir                 string
	manifest            *Manifest
	enabled             bool
	status              PluginStatus
	lastError           string
	consecutiveFailures int
	activeCalls         int
	lastActivity        time.Time
	proc                *pluginProcess
	logs                []string
	logMu               sync.Mutex
	updatedAt           time.Time
}

func (mp *managedPlugin) appendLog(line string) {
	mp.logMu.Lock()
	defer mp.logMu.Unlock()
	if len(mp.logs) >= maxLogLines {
		copy(mp.logs, mp.logs[1:])
		mp.logs[len(mp.logs)-1] = line
	} else {
		mp.logs = append(mp.logs, line)
	}
}

func (mp *managedPlugin) snapshotLogs() []string {
	mp.logMu.Lock()
	defer mp.logMu.Unlock()
	cp := make([]string, len(mp.logs))
	copy(cp, mp.logs)
	return cp
}

// Host supervises installed plugins, manages scale-to-zero lifecycles, and dispatches RPCs.
type Host struct {
	repo               repository.StorageEngine
	pluginsDir         string
	apiBase            string
	shelfdVersion      string
	masterSecret       string
	idleTimeout        time.Duration
	jobWatchdogTimeout time.Duration
	tokenIssuer        TokenIssuer
	hub                *events.Hub
	jobs               *PluginJobManager
	logger             *slog.Logger

	mu      sync.RWMutex
	plugins map[string]*managedPlugin

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHost creates a new Shelfd Plugin Host.
func NewHost(repo repository.StorageEngine, cfg HostConfig) *Host {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	idleTimeout := cfg.IdleTimeout
	if idleTimeout <= 0 {
		idleTimeout = 120 * time.Second
	}
	jobWatchdog := cfg.JobIdleTimeout
	if jobWatchdog <= 0 {
		jobWatchdog = defaultJobWatchdogTimeout
	}
	ctx, cancel := context.WithCancel(context.Background())
	pjm := NewPluginJobManager(cfg.Hub)
	if cfg.JobManager != nil {
		pjm.SetJobManager(cfg.JobManager)
		cfg.JobManager.SetOnDismiss(pjm.RemoveLocal)
	}
	return &Host{
		repo:               repo,
		pluginsDir:         cfg.PluginsDir,
		apiBase:            strings.TrimRight(cfg.APIBase, "/"),
		shelfdVersion:      cfg.ShelfdVersion,
		masterSecret:       cfg.MasterSecret,
		idleTimeout:        idleTimeout,
		jobWatchdogTimeout: jobWatchdog,
		tokenIssuer:        cfg.TokenIssuer,
		hub:                cfg.Hub,
		jobs:               pjm,
		logger:             logger,
		plugins:            make(map[string]*managedPlugin),
		ctx:                ctx,
		cancel:             cancel,
	}
}

// SetJobWatchdogTimeout overrides the inactivity watchdog timeout for background plugin jobs.
func (h *Host) SetJobWatchdogTimeout(d time.Duration) {
	if d > 0 {
		h.mu.Lock()
		h.jobWatchdogTimeout = d
		h.mu.Unlock()
	}
}

// Start discovers installed plugins, probes enabled plugins to cache capabilities, and starts the idle reaper.
func (h *Host) Start(parentCtx context.Context) {
	if err := h.Discover(parentCtx); err != nil {
		h.logger.Warn("plugin discovery warning", "dir", h.pluginsDir, "error", err)
	}

	h.mu.RLock()
	names := make([]string, 0, len(h.plugins))
	for name := range h.plugins {
		names = append(names, name)
	}
	h.mu.RUnlock()

	for _, name := range names {
		h.mu.RLock()
		mp := h.plugins[name]
		h.mu.RUnlock()
		if mp == nil {
			continue
		}
		mp.mu.Lock()
		enabled := mp.enabled
		isDaemon := mp.manifest.Daemon
		mp.mu.Unlock()
		if !enabled {
			continue
		}
		// Probe at boot to validate manifest/capabilities; daemon plugins stay resident, others reap when idle
		if _, err := h.ensureProcess(parentCtx, mp); err != nil {
			h.logger.Warn("plugin startup probe failed", "component", "plugin."+name, "error", err)
		} else if !isDaemon {
			h.logger.Info("plugin initialized and ready (scale-to-zero enabled)", "component", "plugin."+name)
		}
	}

	h.wg.Add(1)
	go h.idleReaperLoop()
}

// Stop gracefully shuts down all running plugin processes.
func (h *Host) Stop() {
	h.cancel()
	h.wg.Wait()

	h.mu.RLock()
	list := make([]*managedPlugin, 0, len(h.plugins))
	for _, mp := range h.plugins {
		list = append(list, mp)
	}
	h.mu.RUnlock()

	for _, mp := range list {
		mp.mu.Lock()
		proc := mp.proc
		mp.proc = nil
		if mp.enabled {
			mp.status = StatusIdle
		}
		mp.mu.Unlock()
		if proc != nil {
			h.stopProcess(proc)
		}
	}
}

// Discover scans pluginsDir for valid plugin directories and reconciles database state.
func (h *Host) Discover(ctx context.Context) error {
	if strings.TrimSpace(h.pluginsDir) == "" {
		return nil
	}
	entries, err := os.ReadDir(h.pluginsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading plugins dir: %w", err)
	}

	savedStates := make(map[string]*repository.PluginState)
	if list, err := h.repo.ListPluginStates(ctx); err == nil {
		for _, st := range list {
			savedStates[st.Name] = st
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pDir := filepath.Join(h.pluginsDir, entry.Name())
		manifest, err := LoadManifest(pDir)
		if err != nil {
			h.logger.Warn("skipping invalid plugin manifest", "dir", pDir, "error", err)
			continue
		}

		enabled := true
		updatedAt := time.Now().UTC()
		if st, ok := savedStates[manifest.Name]; ok {
			enabled = st.Enabled
			updatedAt = st.UpdatedAt
			// Merge cached dynamic tools/commands/actions/auth from prior handshake if present
			if st.CachedManifestJSON != "" && st.CachedManifestJSON != "{}" {
				var cached Manifest
				if json.Unmarshal([]byte(st.CachedManifestJSON), &cached) == nil {
					if len(manifest.Tools) == 0 && len(cached.Tools) > 0 {
						manifest.Tools = cached.Tools
					}
					if len(manifest.Commands) == 0 && len(cached.Commands) > 0 {
						manifest.Commands = cached.Commands
					}
					if manifest.Catalog == nil && cached.Catalog != nil {
						manifest.Catalog = cached.Catalog
					}
					if manifest.Auth == nil && cached.Auth != nil {
						manifest.Auth = cached.Auth
					}
					if len(manifest.Actions) == 0 && len(cached.Actions) > 0 {
						manifest.Actions = cached.Actions
					}
				}
			}
		} else {
			rawManifest, _ := json.Marshal(manifest)
			_ = h.repo.UpsertPluginState(ctx, &repository.PluginState{
				Name:               manifest.Name,
				Enabled:            true,
				InstalledVersion:   manifest.Version,
				CachedManifestJSON: string(rawManifest),
			})
		}

		status := StatusIdle
		if !enabled {
			status = StatusDisabled
		}

		h.mu.Lock()
		if existing, exists := h.plugins[manifest.Name]; exists {
			existing.mu.Lock()
			existing.dir = pDir
			existing.manifest = manifest
			existing.enabled = enabled
			if !enabled {
				existing.status = StatusDisabled
			} else if existing.proc == nil && existing.status != StatusStarting {
				existing.status = StatusIdle
			}
			existing.mu.Unlock()
		} else {
			h.plugins[manifest.Name] = &managedPlugin{
				dir:          pDir,
				manifest:     manifest,
				enabled:      enabled,
				status:       status,
				lastActivity: time.Now(),
				updatedAt:    updatedAt,
			}
		}
		h.mu.Unlock()
	}

	return nil
}

// InstallZipOptions configures how a plugin .zip archive is installed or updated.
type InstallZipOptions struct {
	// ExpectedName, when non-empty, requires the target plugin to already exist
	// and requires the archive's manifest.name to match ExpectedName.
	ExpectedName string
	// UserID is used to populate user-specific fields (e.g. Configured) on the returned summary.
	UserID string
}

// InstallZip extracts a plugin .zip archive into pluginsDir, registers it, and probes its handshake.
func (h *Host) InstallZip(ctx context.Context, archiveBytes []byte) (*PluginSummary, error) {
	res, err := h.InstallOrUpdateZip(ctx, archiveBytes, InstallZipOptions{})
	if err != nil {
		return nil, err
	}
	return &res.PluginSummary, nil
}

// InstallOrUpdateZip stages and validates a plugin .zip archive, stops any existing running
// process before swapping files on disk, preserves enabled state and .data on updates,
// re-discovers the plugin, and probes its handshake if enabled.
func (h *Host) InstallOrUpdateZip(ctx context.Context, archiveBytes []byte, opts InstallZipOptions) (*PluginInstallResult, error) {
	expectedName := strings.TrimSpace(opts.ExpectedName)
	if expectedName != "" && h.getManaged(expectedName) == nil {
		return nil, ErrPluginNotFound
	}

	manifest, stagingDir, err := StagePluginZip(archiveBytes, h.pluginsDir, expectedName)
	if err != nil {
		return nil, err
	}
	activated := false
	defer func() {
		if !activated {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	existing := h.getManaged(manifest.Name)
	wasInstalled := existing != nil
	enabled := true
	previousVersion := ""

	// Stop existing running instance BEFORE swapping files on disk
	if existing != nil {
		existing.mu.Lock()
		enabled = existing.enabled
		if existing.manifest != nil {
			previousVersion = existing.manifest.Version
		}
		oldProc := existing.proc
		existing.proc = nil
		existing.consecutiveFailures = 0
		existing.lastError = ""
		if enabled {
			existing.status = StatusIdle
		} else {
			existing.status = StatusDisabled
		}
		existing.mu.Unlock()
		if oldProc != nil {
			h.stopProcess(oldProc)
		}
	}

	if err := ActivateStagedPlugin(stagingDir, h.pluginsDir, manifest.Name, true); err != nil {
		return nil, err
	}
	activated = true

	rawManifest, _ := json.Marshal(manifest)
	_ = h.repo.UpsertPluginState(ctx, &repository.PluginState{
		Name:               manifest.Name,
		Enabled:            enabled,
		InstalledVersion:   manifest.Version,
		CachedManifestJSON: string(rawManifest),
	})

	if err := h.Discover(ctx); err != nil {
		return nil, err
	}

	mp := h.getManaged(manifest.Name)
	if mp != nil && enabled {
		if _, err := h.ensureProcess(ctx, mp); err != nil {
			h.logger.Warn("installed plugin failed initial handshake", "component", "plugin."+manifest.Name, "error", err)
		}
	}

	sum, err := h.GetPluginSummary(ctx, manifest.Name, opts.UserID)
	if err != nil {
		return nil, err
	}
	return &PluginInstallResult{
		PluginSummary:   *sum,
		Updated:         wasInstalled,
		PreviousVersion: previousVersion,
	}, nil
}

// SetEnabled enables or disables an installed plugin.
func (h *Host) SetEnabled(ctx context.Context, name string, enabled bool, userID string) (*PluginSummary, error) {
	mp := h.getManaged(name)
	if mp == nil {
		return nil, ErrPluginNotFound
	}

	mp.mu.Lock()
	mp.enabled = enabled
	mp.consecutiveFailures = 0
	if !enabled {
		mp.status = StatusDisabled
		mp.lastError = ""
	} else if mp.proc == nil {
		mp.status = StatusIdle
		mp.lastError = ""
	}
	procToStop := (*pluginProcess)(nil)
	if !enabled && mp.proc != nil {
		procToStop = mp.proc
		mp.proc = nil
	}
	rawManifest, _ := json.Marshal(mp.manifest)
	ver := mp.manifest.Version
	mp.updatedAt = time.Now().UTC()
	mp.mu.Unlock()

	if procToStop != nil {
		h.stopProcess(procToStop)
	}

	if err := h.repo.UpsertPluginState(ctx, &repository.PluginState{
		Name:               name,
		Enabled:            enabled,
		InstalledVersion:   ver,
		CachedManifestJSON: string(rawManifest),
	}); err != nil {
		return nil, err
	}

	if enabled {
		if _, err := h.ensureProcess(ctx, mp); err != nil {
			h.logger.Warn("failed probing newly enabled plugin", "component", "plugin."+name, "error", err)
		}
	}

	return h.GetPluginSummary(ctx, name, userID)
}

// ListPlugins returns summaries of all installed plugins for a given user.
func (h *Host) ListPlugins(ctx context.Context, userID string) ([]PluginSummary, error) {
	h.mu.RLock()
	names := make([]string, 0, len(h.plugins))
	for name := range h.plugins {
		names = append(names, name)
	}
	h.mu.RUnlock()
	sort.Strings(names)

	out := make([]PluginSummary, 0, len(names))
	for _, name := range names {
		sum, err := h.GetPluginSummary(ctx, name, userID)
		if err == nil && sum != nil {
			out = append(out, *sum)
		}
	}
	return out, nil
}

// GetPluginSummary returns the runtime status and metadata summary of a single plugin.
func (h *Host) GetPluginSummary(ctx context.Context, name string, userID string) (*PluginSummary, error) {
	mp := h.getManaged(name)
	if mp == nil {
		return nil, ErrPluginNotFound
	}

	configured := true
	mp.mu.Lock()
	schema := mp.manifest.ConfigSchema
	mp.mu.Unlock()

	if hasRequiredConfigFields(schema) {
		configured = false
		if userID != "" {
			if row, err := h.repo.GetPluginUserConfig(ctx, userID, name); err == nil && row != nil && row.ConfigJSON != "" && row.ConfigJSON != "{}" {
				var stored map[string]any
				if json.Unmarshal([]byte(row.ConfigJSON), &stored) == nil && checkRequiredFieldsPresent(schema, stored) {
					configured = true
				}
			}
		}
	}

	mp.mu.Lock()
	defer mp.mu.Unlock()
	m := mp.manifest
	caps := m.Capabilities
	if caps == nil {
		caps = []string{}
	}
	modes := m.CatalogModes
	if len(modes) == 0 && m.Catalog != nil {
		modes = m.Catalog.Modes
	}
	formats := m.SupportedFormats
	if len(formats) == 0 && m.Catalog != nil {
		formats = m.Catalog.SupportedFormats
	}
	updatedAt := mp.updatedAt
	return &PluginSummary{
		Name:             m.Name,
		DisplayName:      m.ResolvedDisplayName(),
		Version:          m.Version,
		Description:      m.Description,
		Homepage:         m.Homepage,
		Enabled:          mp.enabled,
		Daemon:           m.Daemon,
		Status:           mp.status,
		LastError:        mp.lastError,
		Capabilities:     caps,
		CatalogModes:     modes,
		SupportedFormats: formats,
		APIScopes:        m.APIScopes,
		Permissions:      m.Permissions,
		ConfigSchema:     m.ConfigSchema,
		Auth:             m.Auth,
		Tools:            m.Tools,
		Commands:         m.Commands,
		Catalog:          m.Catalog,
		Actions:          m.Actions,
		Configured:       configured,
		UpdatedAt:        &updatedAt,
	}, nil
}

// GetPluginDetail returns PluginSummary plus the recent stderr/log tail.
func (h *Host) GetPluginDetail(ctx context.Context, name string, userID string) (*PluginDetail, error) {
	sum, err := h.GetPluginSummary(ctx, name, userID)
	if err != nil {
		return nil, err
	}
	mp := h.getManaged(name)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	return &PluginDetail{
		PluginSummary: *sum,
		Logs:          mp.snapshotLogs(),
	}, nil
}

// GetRedactedUserConfig returns masked per-user configuration for display in the web/mobile UI.
func (h *Host) GetRedactedUserConfig(ctx context.Context, userID, pluginName string) (map[string]any, []string, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, nil, ErrPluginNotFound
	}
	mp.mu.Lock()
	schema := mp.manifest.ConfigSchema
	mp.mu.Unlock()

	stored := make(map[string]any)
	if row, err := h.repo.GetPluginUserConfig(ctx, userID, pluginName); err == nil && row != nil && row.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(row.ConfigJSON), &stored)
	}
	redacted, secretSet := RedactStoredConfig(schema, stored)
	if secretSet == nil {
		secretSet = []string{}
	}
	return redacted, secretSet, nil
}

// UpdateUserConfig encrypts secrets, applies Smart cURL extraction, persists to DB, and notifies running plugin.
func (h *Host) UpdateUserConfig(ctx context.Context, userID, pluginName string, incoming map[string]any) (map[string]any, []string, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, nil, ErrPluginNotFound
	}
	mp.mu.Lock()
	schema := mp.manifest.ConfigSchema
	proc := mp.proc
	mp.mu.Unlock()

	existing := make(map[string]any)
	if row, err := h.repo.GetPluginUserConfig(ctx, userID, pluginName); err == nil && row != nil && row.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(row.ConfigJSON), &existing)
	}

	stored, decrypted, err := PrepareConfigForStorage(h.masterSecret, pluginName, schema, incoming, existing)
	if err != nil {
		return nil, nil, err
	}

	rawStored, err := json.Marshal(stored)
	if err != nil {
		return nil, nil, err
	}
	if err := h.repo.UpsertPluginUserConfig(ctx, &repository.PluginUserConfig{
		UserID:     userID,
		PluginName: pluginName,
		ConfigJSON: string(rawStored),
	}); err != nil {
		return nil, nil, err
	}

	if proc != nil {
		_ = proc.notify("config/update", map[string]any{
			"user_id": userID,
			"config":  decrypted,
		})
	}

	redacted, secretSet := RedactStoredConfig(schema, stored)
	if secretSet == nil {
		secretSet = []string{}
	}
	return redacted, secretSet, nil
}

// AvailableMCPTools returns aggregated MCP tool definitions across all enabled plugins as "{plugin}_{tool}".
func (h *Host) AvailableMCPTools() []ToolDef {
	h.mu.RLock()
	defer h.mu.RUnlock()

	names := make([]string, 0, len(h.plugins))
	for name := range h.plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []ToolDef
	for _, name := range names {
		mp := h.plugins[name]
		mp.mu.Lock()
		enabled := mp.enabled
		tools := mp.manifest.Tools
		mp.mu.Unlock()
		if !enabled {
			continue
		}
		for _, t := range tools {
			schema := t.InputSchema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			out = append(out, ToolDef{
				Name:        name + "_" + t.Name,
				Description: t.Description,
				InputSchema: schema,
			})
		}
	}
	return out
}

// CallMCPTool dispatches a "{plugin}_{tool}" invocation to the owning plugin.
func (h *Host) CallMCPTool(ctx context.Context, qualifiedName string, args map[string]any, userID, username string) (*ToolCallResult, bool, error) {
	mp, localToolName := h.resolveMCPTool(qualifiedName)
	if mp == nil {
		return nil, false, nil
	}

	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, true, err
	}

	rawRes, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "tools/call", ToolCallParams{
		Name:      localToolName,
		Arguments: args,
		Auth:      auth,
	})
	if err != nil {
		return nil, true, err
	}

	var res ToolCallResult
	if err := json.Unmarshal(rawRes, &res); err != nil {
		// Allow plugins returning standard MCP content array format as well
		var mcpStyle struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		}
		if json.Unmarshal(rawRes, &mcpStyle) == nil && len(mcpStyle.Content) > 0 {
			var parts []string
			for _, c := range mcpStyle.Content {
				if c.Text != "" {
					parts = append(parts, c.Text)
				}
			}
			res = ToolCallResult{Content: strings.Join(parts, "\n"), IsError: mcpStyle.IsError}
		} else {
			res = ToolCallResult{Content: string(rawRes)}
		}
	}

	// Enforce MCP LOD ceiling (cap single plugin tool output at 16 KB so external agent contexts stay protected)
	const maxToolTextBytes = 16 << 10
	if len(res.Content) > maxToolTextBytes {
		res.Content = res.Content[:maxToolTextBytes] + "\n...[truncated to enforce MCP LOD token ceiling]"
	}

	return &res, true, nil
}

// InvokeCommand dispatches a user-triggered command via REST.
func (h *Host) InvokeCommand(ctx context.Context, pluginName, command string, args map[string]any, userID, username string) (json.RawMessage, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	return h.invokeRPC(ctx, mp, defaultCallTimeout, "commands/invoke", CommandInvokeParams{
		Command: command,
		Args:    args,
		Auth:    auth,
	})
}

// BrowseCatalog invokes catalog/browse and enriches returned items with staged/in_library status.
func (h *Host) BrowseCatalog(ctx context.Context, pluginName, group, cursor, filter, userID, username string) (*CatalogPage, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "catalog/browse", CatalogBrowseParams{
		Group:  group,
		Cursor: cursor,
		Filter: filter,
		Auth:   auth,
	})
	if err != nil {
		return nil, err
	}
	var page CatalogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding catalog/browse response: %w", err)
	}
	h.enrichCatalogPage(ctx, pluginName, &page)
	return &page, nil
}

// SearchCatalog invokes catalog/search and enriches returned items with staged/in_library status.
func (h *Host) SearchCatalog(ctx context.Context, pluginName, query, cursor string, filters map[string]any, userID, username string) (*CatalogPage, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "catalog/search", CatalogSearchParams{
		Query:   query,
		Cursor:  cursor,
		Filters: filters,
		Auth:    auth,
	})
	if err != nil {
		return nil, err
	}
	var page CatalogPage
	if err := json.Unmarshal(raw, &page); err != nil {
		return nil, fmt.Errorf("decoding catalog/search response: %w", err)
	}
	h.enrichCatalogPage(ctx, pluginName, &page)
	return &page, nil
}

// StageCatalogItems invokes catalog/stage so the plugin downloads and stages selected items via loopback StageUploadBook.
func (h *Host) StageCatalogItems(ctx context.Context, pluginName string, items []CatalogStageItem, forceAsync bool, userID, username string) (*CatalogStageResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}

	if !forceAsync {
		raw, err := h.invokeRPC(ctx, mp, stageCallTimeout, "catalog/stage", CatalogStageParams{
			Items: items,
			Auth:  auth,
		})
		if err != nil {
			return nil, err
		}
		var res CatalogStageResult
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("decoding catalog/stage response: %w", err)
		}
		return &res, nil
	}

	// Ensure process is running and acquire a scale-to-zero lease before returning 202 Accepted
	mp.mu.Lock()
	if !mp.enabled {
		mp.mu.Unlock()
		return nil, ErrPluginDisabled
	}
	mp.activeCalls++
	mp.lastActivity = time.Now()
	mp.mu.Unlock()

	proc, err := h.ensureProcess(ctx, mp)
	if err != nil {
		mp.mu.Lock()
		mp.activeCalls--
		mp.lastActivity = time.Now()
		mp.mu.Unlock()
		return nil, err
	}

	externalIDs := make([]string, 0, len(items))
	for _, it := range items {
		ext := strings.TrimSpace(it.ExternalID)
		if ext != "" {
			externalIDs = append(externalIDs, ext)
		}
	}
	firstExtID := ""
	if len(externalIDs) > 0 {
		firstExtID = externalIDs[0]
	}

	var jobTitle string
	if len(items) == 1 {
		trimmedTitle := strings.TrimSpace(items[0].Title)
		if trimmedTitle != "" {
			jobTitle = fmt.Sprintf("Staging %q", trimmedTitle)
		} else {
			jobTitle = "Staging 1 book"
		}
	} else {
		jobTitle = fmt.Sprintf("Staging %d books", len(items))
	}

	target := ActionTarget{
		Type:  "catalog",
		ID:    firstExtID,
		Title: jobTitle,
	}
	job, touchCh := h.jobs.CreateJobWithTotal(userID, pluginName, "catalog_stage", jobTitle, target, len(items), externalIDs)

	h.mu.RLock()
	watchdogDur := h.jobWatchdogTimeout
	h.mu.RUnlock()

	itemsCopy := make([]CatalogStageItem, len(items))
	copy(itemsCopy, items)

	go func() {
		defer func() {
			mp.mu.Lock()
			mp.activeCalls--
			mp.lastActivity = time.Now()
			mp.mu.Unlock()
		}()

		jobCtx, cancelJob := context.WithCancel(h.ctx)
		defer cancelJob()

		doneRPC := make(chan struct{})
		go func() {
			timer := time.NewTimer(watchdogDur)
			defer timer.Stop()
			for {
				select {
				case <-doneRPC:
					return
				case <-jobCtx.Done():
					return
				case <-touchCh:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(watchdogDur)
				case <-timer.C:
					cancelJob()
					return
				}
			}
		}()

		raw, callErr := proc.call(jobCtx, "catalog/stage", CatalogStageParams{
			JobID: job.ID,
			Items: itemsCopy,
			Auth:  auth,
		})
		close(doneRPC)

		if callErr != nil {
			errMsg := callErr.Error()
			if errors.Is(callErr, context.Canceled) && h.ctx.Err() == nil {
				errMsg = fmt.Sprintf("plugin job timed out after %s without progress activity", watchdogDur)
			}
			h.jobs.FailJob(job.ID, errMsg)
			return
		}

		var res CatalogStageResult
		if err := json.Unmarshal(raw, &res); err != nil {
			h.jobs.FailJob(job.ID, fmt.Sprintf("invalid catalog/stage response: %v", err))
			return
		}

		stagedCount := 0
		failedCount := 0
		var failedMsgs []string
		for _, entry := range res.Results {
			if strings.EqualFold(entry.Status, "failed") {
				failedCount++
				label := strings.TrimSpace(entry.Title)
				if label == "" {
					label = entry.ExternalID
				}
				errDetail := "Failed"
				if entry.Error != nil && strings.TrimSpace(*entry.Error) != "" {
					errDetail = strings.TrimSpace(*entry.Error)
				}
				if label != "" {
					failedMsgs = append(failedMsgs, fmt.Sprintf("%s: %s", label, errDetail))
				} else {
					failedMsgs = append(failedMsgs, errDetail)
				}
			} else {
				stagedCount++
			}
		}

		if len(res.Results) > 0 && stagedCount == 0 {
			errSummary := strings.Join(failedMsgs, "; ")
			if errSummary == "" {
				errSummary = "Failed to stage selected catalog items"
			}
			h.jobs.FailJob(job.ID, errSummary)
			return
		}

		summaryMsg := strings.TrimSpace(res.Message)
		if summaryMsg == "" {
			if failedCount > 0 {
				totalReported := stagedCount + failedCount
				bookWord := "books"
				if totalReported == 1 {
					bookWord = "book"
				}
				summaryMsg = fmt.Sprintf("Staged %d of %d %s (%d failed)", stagedCount, totalReported, bookWord, failedCount)
			} else if stagedCount == 1 {
				summaryMsg = "Staged 1 book for review"
			} else if stagedCount > 1 {
				summaryMsg = fmt.Sprintf("Staged %d books for review", stagedCount)
			} else {
				summaryMsg = "Completed staging"
			}
		}

		batchID := res.BatchID
		if batchID == "" {
			batchID = job.ID
		}
		h.jobs.CompleteJob(job.ID, &ActionResult{
			Status:  "completed",
			Message: summaryMsg,
			Data: map[string]any{
				"batch_id":     batchID,
				"results":      res.Results,
				"staged_count": stagedCount,
				"failed_count": failedCount,
			},
		})
	}()

	return &CatalogStageResult{
		BatchID: job.ID,
		Async:   true,
		JobID:   job.ID,
		Job:     job,
		Message: "Staging started in background",
	}, nil
}

// GetAuthStatus invokes auth/status on a plugin to check whether the user's account is linked.
func (h *Host) GetAuthStatus(ctx context.Context, pluginName, userID, username string) (*AuthStatusResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "auth/status", AuthStatusParams{
		Auth: auth,
	})
	if err != nil {
		return nil, err
	}
	var res AuthStatusResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decoding auth/status response: %w", err)
	}
	return &res, nil
}

// BeginAuth invokes auth/begin on a plugin to start an OAuth 2.0 PKCE or interactive login flow.
func (h *Host) BeginAuth(ctx context.Context, pluginName string, params AuthBeginParams, userID, username string) (*AuthBeginResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	params.Auth = auth
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "auth/begin", params)
	if err != nil {
		return nil, err
	}
	var res AuthBeginResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decoding auth/begin response: %w", err)
	}
	return &res, nil
}

// CompleteAuth invokes auth/complete on a plugin to exchange an OAuth code or persist tokens.
func (h *Host) CompleteAuth(ctx context.Context, pluginName string, params AuthCompleteParams, userID, username string) (*AuthStatusResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	params.Auth = auth
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "auth/complete", params)
	if err != nil {
		return nil, err
	}
	var res AuthStatusResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decoding auth/complete response: %w", err)
	}
	return &res, nil
}

// DisconnectAuth invokes auth/disconnect on a plugin to unlink the user's account credentials.
func (h *Host) DisconnectAuth(ctx context.Context, pluginName, userID, username string) (*AuthStatusResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "auth/disconnect", AuthDisconnectParams{
		Auth: auth,
	})
	if err != nil {
		return nil, err
	}
	var res AuthStatusResult
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &res)
	}
	return &res, nil
}

// PrepareAction invokes actions/prepare on a plugin to build a dynamic ActionFormSpec for a target book or series.
func (h *Host) PrepareAction(ctx context.Context, pluginName, actionName string, target ActionTarget, reqCtx map[string]any, userID, username string) (*ActionFormSpec, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "actions/prepare", ActionPrepareParams{
		Action:  actionName,
		Target:  target,
		Context: reqCtx,
		Auth:    auth,
	})
	if err != nil {
		return nil, err
	}
	var spec ActionFormSpec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("decoding actions/prepare response: %w", err)
	}
	if spec.Fields == nil {
		spec.Fields = []InputFieldSpec{}
	}
	return &spec, nil
}

// FetchActionOptions invokes actions/options on a plugin to populate remote_select or asset_picker options.
func (h *Host) FetchActionOptions(ctx context.Context, pluginName, actionName, source, tab, query, cursor string, target *ActionTarget, userID, username string) (*ActionOptionsResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "actions/options", ActionOptionsParams{
		Action: actionName,
		Source: source,
		Tab:    tab,
		Query:  query,
		Cursor: cursor,
		Target: target,
		Auth:   auth,
	})
	if err != nil {
		return nil, err
	}
	var res ActionOptionsResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decoding actions/options response: %w", err)
	}
	if res.Items == nil {
		res.Items = []OptionItem{}
	}
	return &res, nil
}

// UploadActionAsset invokes actions/upload_asset on a plugin with base64-encoded asset bytes.
func (h *Host) UploadActionAsset(ctx context.Context, pluginName, actionName, field, filename, mimeType, dataBase64, userID, username string) (*AssetUploadResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}
	raw, err := h.invokeRPC(ctx, mp, defaultCallTimeout, "actions/upload_asset", AssetUploadParams{
		Action:     actionName,
		Field:      field,
		Filename:   filename,
		MimeType:   mimeType,
		DataBase64: dataBase64,
		Auth:       auth,
	})
	if err != nil {
		return nil, err
	}
	var res AssetUploadResult
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decoding actions/upload_asset response: %w", err)
	}
	return &res, nil
}

// InvokeAction executes a plugin action either synchronously or as a background tracked PluginJob.
func (h *Host) InvokeAction(ctx context.Context, pluginName, actionName string, target ActionTarget, values map[string]any, overrides map[string]string, forceAsync bool, userID, username string) (*ActionResult, error) {
	mp := h.getManaged(pluginName)
	if mp == nil {
		return nil, ErrPluginNotFound
	}
	auth, err := h.buildInvocationAuth(ctx, mp, userID, username)
	if err != nil {
		return nil, err
	}

	mp.mu.Lock()
	var matchedDef *ActionDef
	for i := range mp.manifest.Actions {
		if mp.manifest.Actions[i].Name == actionName {
			defCopy := mp.manifest.Actions[i]
			matchedDef = &defCopy
			break
		}
	}
	mp.mu.Unlock()

	runAsJob := forceAsync || (matchedDef != nil && strings.EqualFold(matchedDef.Execution, "job"))
	if !runAsJob {
		raw, err := h.invokeRPC(ctx, mp, stageCallTimeout, "actions/invoke", ActionInvokeParams{
			Action:    actionName,
			Target:    target,
			Values:    values,
			Overrides: overrides,
			Auth:      auth,
		})
		if err != nil {
			return nil, err
		}
		var res ActionResult
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("decoding actions/invoke response: %w", err)
		}
		if res.Status == "" {
			res.Status = "completed"
		}
		return &res, nil
	}

	// Ensure process is running and acquire a scale-to-zero lease before returning 202 Accepted
	mp.mu.Lock()
	if !mp.enabled {
		mp.mu.Unlock()
		return nil, ErrPluginDisabled
	}
	mp.activeCalls++
	mp.lastActivity = time.Now()
	mp.mu.Unlock()

	proc, err := h.ensureProcess(ctx, mp)
	if err != nil {
		mp.mu.Lock()
		mp.activeCalls--
		mp.lastActivity = time.Now()
		mp.mu.Unlock()
		return nil, err
	}

	jobTitle := target.Title
	if jobTitle == "" && matchedDef != nil && matchedDef.Title != "" {
		jobTitle = matchedDef.Title
	}
	job, touchCh := h.jobs.CreateJob(userID, pluginName, actionName, jobTitle, target)

	h.mu.RLock()
	watchdogDur := h.jobWatchdogTimeout
	h.mu.RUnlock()

	go func() {
		defer func() {
			mp.mu.Lock()
			mp.activeCalls--
			mp.lastActivity = time.Now()
			mp.mu.Unlock()
		}()

		jobCtx, cancelJob := context.WithCancel(h.ctx)
		defer cancelJob()

		doneRPC := make(chan struct{})
		go func() {
			timer := time.NewTimer(watchdogDur)
			defer timer.Stop()
			for {
				select {
				case <-doneRPC:
					return
				case <-jobCtx.Done():
					return
				case <-touchCh:
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(watchdogDur)
				case <-timer.C:
					cancelJob()
					return
				}
			}
		}()

		raw, callErr := proc.call(jobCtx, "actions/invoke", ActionInvokeParams{
			Action:    actionName,
			JobID:     job.ID,
			Target:    target,
			Values:    values,
			Overrides: overrides,
			Auth:      auth,
		})
		close(doneRPC)

		if callErr != nil {
			errMsg := callErr.Error()
			if errors.Is(callErr, context.Canceled) && h.ctx.Err() == nil {
				errMsg = fmt.Sprintf("plugin job timed out after %s without progress activity", watchdogDur)
			}
			h.jobs.FailJob(job.ID, errMsg)
			return
		}

		var res ActionResult
		if err := json.Unmarshal(raw, &res); err != nil {
			h.jobs.FailJob(job.ID, fmt.Sprintf("invalid actions/invoke response: %v", err))
			return
		}
		if res.Status == "" {
			res.Status = "completed"
		}
		h.jobs.CompleteJob(job.ID, &res)
	}()

	return &ActionResult{
		Status:  "job_started",
		JobID:   job.ID,
		Job:     job,
		Message: "Background job started",
	}, nil
}

// ListUserJobs returns all tracked background plugin jobs for a user.
func (h *Host) ListUserJobs(userID string) []PluginJob {
	return h.jobs.ListUserJobs(userID)
}

// GetJob returns a single tracked background plugin job by ID.
func (h *Host) GetJob(jobID string) *PluginJob {
	return h.jobs.GetJob(jobID)
}

// DismissJob deletes a tracked background plugin job for a user.
func (h *Host) DismissJob(userID, jobID string) bool {
	return h.jobs.DismissJob(userID, jobID)
}

func (h *Host) enrichCatalogPage(ctx context.Context, pluginName string, page *CatalogPage) {
	if page == nil || len(page.Items) == 0 {
		return
	}
	extIDs := make([]string, 0, len(page.Items))
	for _, it := range page.Items {
		if it.ExternalID != "" {
			extIDs = append(extIDs, it.ExternalID)
		}
	}
	sources, err := h.repo.GetBookSourcesByExternalIDs(ctx, pluginName, extIDs)
	srcMap := make(map[string]*repository.BookSource, len(sources))
	if err == nil {
		for _, s := range sources {
			srcMap[s.ExternalID] = s
		}
	}

	for i := range page.Items {
		it := &page.Items[i]
		src, ok := srcMap[it.ExternalID]
		if !ok || src == nil {
			it.Status = "available"
			continue
		}
		if src.BookID != nil && *src.BookID != "" {
			if book, bErr := h.repo.GetBookByID(ctx, *src.BookID); bErr == nil && book != nil {
				it.Status = "in_library"
				it.BookID = src.BookID
				continue
			}
		}
		if src.UploadJobID != nil && *src.UploadJobID != "" {
			if job, jErr := h.repo.GetUploadJob(ctx, *src.UploadJobID); jErr == nil && job != nil {
				if job.Status == "completed" && job.BookID != nil {
					it.Status = "in_library"
					it.BookID = job.BookID
					continue
				}
				if job.Status == "staged" || job.Status == "queued" || job.Status == "processing" {
					it.Status = "staged"
					it.UploadJobID = src.UploadJobID
					continue
				}
			}
		}
		it.Status = "available"
	}
}

func (h *Host) resolveMCPTool(qualifiedName string) (*managedPlugin, string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for name, mp := range h.plugins {
		prefix := name + "_"
		if strings.HasPrefix(qualifiedName, prefix) {
			local := strings.TrimPrefix(qualifiedName, prefix)
			mp.mu.Lock()
			enabled := mp.enabled
			var matched bool
			for _, t := range mp.manifest.Tools {
				if t.Name == local {
					matched = true
					break
				}
			}
			mp.mu.Unlock()
			if enabled && matched {
				return mp, local
			}
		}
	}
	return nil, ""
}

func (h *Host) getManaged(name string) *managedPlugin {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.plugins[name]
}

func (h *Host) buildInvocationAuth(ctx context.Context, mp *managedPlugin, userID, username string) (InvocationAuth, error) {
	mp.mu.Lock()
	if !mp.enabled {
		mp.mu.Unlock()
		return InvocationAuth{}, ErrPluginDisabled
	}
	pluginName := mp.manifest.Name
	scopes := mp.manifest.APIScopes
	schema := mp.manifest.ConfigSchema
	mp.mu.Unlock()

	decryptedCfg := make(map[string]any)
	if userID != "" && h.masterSecret != "" {
		if row, err := h.repo.GetPluginUserConfig(ctx, userID, pluginName); err == nil && row != nil && row.ConfigJSON != "" {
			var stored map[string]any
			if json.Unmarshal([]byte(row.ConfigJSON), &stored) == nil {
				if dec, _, dErr := DecryptStoredConfig(h.masterSecret, pluginName, schema, stored); dErr == nil {
					decryptedCfg = dec
				}
			}
		}
	}

	var invToken string
	if h.tokenIssuer != nil && userID != "" {
		tok, err := h.tokenIssuer(userID, username, pluginName, scopes)
		if err != nil {
			return InvocationAuth{}, fmt.Errorf("issuing plugin invocation token: %w", err)
		}
		invToken = tok
	}

	return InvocationAuth{
		UserID:          userID,
		InvocationToken: invToken,
		APIBase:         h.apiBase,
		Config:          decryptedCfg,
	}, nil
}

func (h *Host) invokeRPC(ctx context.Context, mp *managedPlugin, timeout time.Duration, method string, params any) (json.RawMessage, error) {
	mp.mu.Lock()
	if !mp.enabled {
		mp.mu.Unlock()
		return nil, ErrPluginDisabled
	}
	mp.activeCalls++
	mp.lastActivity = time.Now()
	mp.mu.Unlock()

	defer func() {
		mp.mu.Lock()
		mp.activeCalls--
		mp.lastActivity = time.Now()
		mp.mu.Unlock()
	}()

	proc, err := h.ensureProcess(ctx, mp)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return proc.call(callCtx, method, params)
}

func (h *Host) ensureProcess(ctx context.Context, mp *managedPlugin) (*pluginProcess, error) {
	mp.mu.Lock()
	if !mp.enabled {
		mp.mu.Unlock()
		return nil, ErrPluginDisabled
	}
	if mp.proc != nil {
		select {
		case <-mp.proc.doneCh:
			mp.proc = nil
		default:
			proc := mp.proc
			mp.lastActivity = time.Now()
			mp.mu.Unlock()
			return proc, nil
		}
	}
	if mp.status == StatusUnhealthy && mp.consecutiveFailures >= 5 {
		lastErr := mp.lastError
		mp.mu.Unlock()
		return nil, fmt.Errorf("plugin is parked unhealthy after repeated crashes: %s", lastErr)
	}

	mp.status = StatusStarting
	pDir := mp.dir
	manifest := mp.manifest
	mp.mu.Unlock()

	proc, initRes, err := h.spawnAndHandshake(ctx, mp, pDir, manifest)
	if err != nil {
		mp.mu.Lock()
		mp.consecutiveFailures++
		mp.lastError = err.Error()
		if mp.consecutiveFailures >= 5 {
			mp.status = StatusUnhealthy
		} else {
			mp.status = StatusIdle
		}
		mp.mu.Unlock()
		return nil, err
	}

	mp.mu.Lock()
	mp.proc = proc
	mp.status = StatusRunning
	mp.consecutiveFailures = 0
	mp.lastError = ""
	mp.lastActivity = time.Now()
	if len(initRes.Tools) > 0 {
		mp.manifest.Tools = initRes.Tools
	}
	if len(initRes.Commands) > 0 {
		mp.manifest.Commands = initRes.Commands
	}
	if initRes.Catalog != nil {
		mp.manifest.Catalog = initRes.Catalog
		if len(mp.manifest.CatalogModes) == 0 {
			mp.manifest.CatalogModes = initRes.Catalog.Modes
		}
		if len(mp.manifest.SupportedFormats) == 0 {
			mp.manifest.SupportedFormats = initRes.Catalog.SupportedFormats
		}
	}
	if initRes.Auth != nil && mp.manifest.Auth == nil {
		mp.manifest.Auth = initRes.Auth
	}
	if len(initRes.Actions) > 0 && len(mp.manifest.Actions) == 0 {
		mp.manifest.Actions = initRes.Actions
	}
	rawCached, _ := json.Marshal(mp.manifest)
	name := mp.manifest.Name
	ver := mp.manifest.Version
	enabled := mp.enabled
	mp.mu.Unlock()

	_ = h.repo.UpsertPluginState(ctx, &repository.PluginState{
		Name:               name,
		Enabled:            enabled,
		InstalledVersion:   ver,
		CachedManifestJSON: string(rawCached),
	})

	return proc, nil
}

func (h *Host) spawnAndHandshake(ctx context.Context, mp *managedPlugin, pluginDir string, manifest *Manifest) (*pluginProcess, *InitializeResult, error) {
	scratchDir := filepath.Join(pluginDir, ".data")
	if err := os.MkdirAll(scratchDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("creating plugin scratch dir: %w", err)
	}

	cmdStr := manifest.Entrypoint.Command
	if strings.HasPrefix(cmdStr, "./") || strings.HasPrefix(cmdStr, "/") {
		if strings.HasPrefix(cmdStr, "./") {
			cmdStr = filepath.Join(pluginDir, strings.TrimPrefix(cmdStr, "./"))
		}
	} else {
		resolved, err := exec.LookPath(cmdStr)
		if err != nil {
			return nil, nil, fmt.Errorf("resolving entrypoint %q on PATH: %w", cmdStr, err)
		}
		cmdStr = resolved
	}

	// Use background host context for process lifetime so individual HTTP request cancellations do not kill the shared plugin process
	cmd := exec.CommandContext(h.ctx, cmdStr, manifest.Entrypoint.Args...)
	cmd.Dir = pluginDir
	cmd.Env = buildCleanPluginEnv(scratchDir)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("creating stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, fmt.Errorf("creating stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		return nil, nil, fmt.Errorf("creating stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("spawning plugin %q: %w", manifest.Name, err)
	}

	proc := &pluginProcess{
		cmd:     cmd,
		stdin:   stdin,
		pending: make(map[string]chan RPCMessage),
		doneCh:  make(chan struct{}),
	}

	go h.readStderrLoop(mp, manifest.Name, stderr)
	go h.readStdoutLoop(mp, manifest.Name, proc, stdout)
	go h.waitProcessLoop(mp, manifest.Name, proc)

	initCtx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()

	rawInit, err := proc.call(initCtx, "initialize", InitializeParams{
		ProtocolVersion: ProtocolVersion,
		ShelfdVersion:   h.shelfdVersion,
		APIBase:         h.apiBase,
		DataDir:         scratchDir,
	})
	if err != nil {
		h.stopProcess(proc)
		return nil, nil, fmt.Errorf("plugin handshake failed: %w", err)
	}

	var initRes InitializeResult
	if err := json.Unmarshal(rawInit, &initRes); err != nil {
		h.stopProcess(proc)
		return nil, nil, fmt.Errorf("invalid initialize response: %w", err)
	}

	if initRes.Auth != nil && manifest.Auth == nil {
		manifest.Auth = initRes.Auth
	}
	if len(initRes.Actions) > 0 && len(manifest.Actions) == 0 {
		manifest.Actions = initRes.Actions
	}

	return proc, &initRes, nil
}

func (h *Host) readStderrLoop(mp *managedPlugin, pluginName string, stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		mp.appendLog(line)
		h.logger.Info("plugin stderr", "component", "plugin."+pluginName, "line", line)
	}
}

func (h *Host) readStdoutLoop(mp *managedPlugin, pluginName string, proc *pluginProcess, stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxRPCFrameBytes)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var msg RPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			h.logger.Warn("malformed ndjson frame from plugin", "component", "plugin."+pluginName, "error", err)
			continue
		}

		if len(msg.ID) > 0 && msg.Method == "" {
			var idKey string
			if json.Unmarshal(msg.ID, &idKey) != nil {
				idKey = strings.Trim(string(msg.ID), `"`)
			}
			proc.pendMu.Lock()
			ch, ok := proc.pending[idKey]
			proc.pendMu.Unlock()
			if ok {
				select {
				case ch <- msg:
				default:
				}
			}
			continue
		}

		if msg.Method != "" {
			go h.handlePluginInbound(mp, pluginName, proc, msg)
		}
	}
}

func (h *Host) handlePluginInbound(mp *managedPlugin, pluginName string, proc *pluginProcess, msg RPCMessage) {
	switch msg.Method {
	case "config/patch":
		var params struct {
			UserID string         `json:"user_id"`
			Values map[string]any `json:"values"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil || params.UserID == "" {
			if len(msg.ID) > 0 {
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &RPCError{Code: -32602, Message: "user_id and values are required"},
				})
			}
			return
		}
		_, _, err := h.UpdateUserConfig(h.ctx, params.UserID, pluginName, params.Values)
		if len(msg.ID) > 0 {
			if err != nil {
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &RPCError{Code: -32603, Message: err.Error()},
				})
			} else {
				okRaw, _ := json.Marshal(map[string]any{"ok": true})
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Result:  okRaw,
				})
			}
		}

	case "host/refresh_token":
		var params struct {
			UserID string `json:"user_id"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil || strings.TrimSpace(params.UserID) == "" {
			h.logger.Warn("refresh_token rejected: missing user_id", "component", "plugin."+pluginName)
			if len(msg.ID) > 0 {
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &RPCError{Code: -32602, Message: "user_id is required"},
				})
			}
			return
		}
		h.logger.Info("refreshing invocation token", "component", "plugin."+pluginName, "user_id", strings.TrimSpace(params.UserID))
		mp.mu.Lock()
		scopes := mp.manifest.APIScopes
		mp.lastActivity = time.Now().UTC()
		mp.mu.Unlock()

		if h.tokenIssuer == nil {
			h.logger.Warn("refresh_token rejected: token issuer not configured", "component", "plugin."+pluginName, "user_id", strings.TrimSpace(params.UserID))
			if len(msg.ID) > 0 {
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &RPCError{Code: -32603, Message: "token issuer is not configured"},
				})
			}
			return
		}
		tok, err := h.tokenIssuer(strings.TrimSpace(params.UserID), "", pluginName, scopes)
		if len(msg.ID) > 0 {
			if err != nil {
				h.logger.Warn("refresh_token failed", "component", "plugin."+pluginName, "user_id", strings.TrimSpace(params.UserID), "error", err.Error())
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   &RPCError{Code: -32603, Message: err.Error()},
				})
			} else {
				h.logger.Info("issued refreshed invocation token", "component", "plugin."+pluginName, "user_id", strings.TrimSpace(params.UserID))
				resRaw, _ := json.Marshal(map[string]any{"invocation_token": tok})
				_ = proc.sendJSON(RPCMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Result:  resRaw,
				})
			}
		}

	case "event", "progress":
		if len(msg.Params) > maxEventPayloadBytes {
			h.logger.Warn("dropping oversized plugin event", "component", "plugin."+pluginName, "bytes", len(msg.Params))
			return
		}
		mp.mu.Lock()
		mp.lastActivity = time.Now().UTC()
		mp.mu.Unlock()

		var payload map[string]any
		if json.Unmarshal(msg.Params, &payload) != nil {
			payload = map[string]any{"raw": string(msg.Params)}
		}

		if msg.Method == "progress" {
			var pMsg struct {
				JobID   string   `json:"job_id"`
				BatchID string   `json:"batch_id"`
				Current int      `json:"current"`
				Total   int      `json:"total"`
				Title   string   `json:"title"`
				Message string   `json:"message"`
				Percent *float64 `json:"percent"`
			}
			if json.Unmarshal(msg.Params, &pMsg) == nil {
				lookupID := strings.TrimSpace(pMsg.JobID)
				if lookupID == "" {
					lookupID = strings.TrimSpace(pMsg.BatchID)
				}
				pct := -1.0
				if pMsg.Percent != nil {
					pct = *pMsg.Percent
				}
				if updated := h.jobs.UpdateProgress(pluginName, lookupID, pMsg.Current, pMsg.Total, pMsg.Title, pMsg.Message, pct); updated != nil {
					return
				}
			}
		}

		payload["plugin"] = pluginName
		payload["kind"] = msg.Method
		if h.hub != nil {
			h.hub.Broadcast(events.Event{
				Type: events.EventPlugin,
				Data: payload,
			})
		}

	case "log":
		var logMsg struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		}
		if json.Unmarshal(msg.Params, &logMsg) == nil && logMsg.Message != "" {
			mp.appendLog(fmt.Sprintf("[%s] %s", strings.ToUpper(logMsg.Level), logMsg.Message))
			h.logger.Info("plugin log", "component", "plugin."+pluginName, "level", logMsg.Level, "msg", logMsg.Message)
		}
	}
}

func (h *Host) waitProcessLoop(mp *managedPlugin, pluginName string, proc *pluginProcess) {
	err := proc.cmd.Wait()
	close(proc.doneCh)

	wasIntentional := proc.stopping.Load() || h.ctx.Err() != nil

	mp.mu.Lock()
	if mp.proc == proc {
		mp.proc = nil
	}
	isDaemon := mp.manifest.Daemon
	enabled := mp.enabled
	if !enabled {
		mp.status = StatusDisabled
		mp.mu.Unlock()
		return
	}

	if wasIntentional {
		mp.status = StatusIdle
		mp.mu.Unlock()
		return
	}

	mp.consecutiveFailures++
	if err != nil {
		mp.lastError = fmt.Sprintf("process exited unexpectedly: %v", err)
	} else {
		mp.lastError = "process exited unexpectedly with status 0"
	}
	failures := mp.consecutiveFailures
	if failures >= 5 {
		mp.status = StatusUnhealthy
		mp.mu.Unlock()
		h.logger.Error("plugin parked as unhealthy after 5 consecutive failures", "component", "plugin."+pluginName)
		return
	}
	mp.status = StatusIdle
	mp.mu.Unlock()

	if isDaemon {
		backoff := time.Duration(1<<uint(failures-1)) * time.Second
		if backoff > 60*time.Second {
			backoff = 60 * time.Second
		}
		h.logger.Warn("daemon plugin exited; scheduling restart", "component", "plugin."+pluginName, "backoff", backoff, "failures", failures)
		go func() {
			select {
			case <-h.ctx.Done():
				return
			case <-time.After(backoff):
				_, _ = h.ensureProcess(h.ctx, mp)
			}
		}()
	}
}

func (h *Host) stopProcess(proc *pluginProcess) {
	if proc == nil {
		return
	}
	proc.stopping.Store(true)
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownGracePeriod)
	_, _ = proc.call(shutCtx, "shutdown", map[string]any{})
	cancel()
	_ = proc.stdin.Close()

	select {
	case <-proc.doneCh:
	case <-time.After(shutdownGracePeriod):
		if proc.cmd.Process != nil {
			_ = proc.cmd.Process.Kill()
		}
		<-proc.doneCh
	}
}

func (h *Host) idleReaperLoop() {
	defer h.wg.Done()
	interval := h.idleTimeout / 4
	if interval < 500*time.Millisecond {
		interval = 500 * time.Millisecond
	}
	if interval > 15*time.Second {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.reapIdleProcesses()
		}
	}
}

func (h *Host) reapIdleProcesses() {
	h.mu.RLock()
	list := make([]*managedPlugin, 0, len(h.plugins))
	for _, mp := range h.plugins {
		list = append(list, mp)
	}
	h.mu.RUnlock()

	now := time.Now()
	for _, mp := range list {
		mp.mu.Lock()
		if mp.proc == nil || mp.manifest.Daemon || mp.activeCalls > 0 || now.Sub(mp.lastActivity) < h.idleTimeout {
			mp.mu.Unlock()
			continue
		}
		proc := mp.proc
		mp.proc = nil
		mp.status = StatusIdle
		name := mp.manifest.Name
		mp.mu.Unlock()

		h.logger.Debug("reaping idle plugin process", "component", "plugin."+name)
		h.stopProcess(proc)
	}
}

// buildCleanPluginEnv builds a secret-free child environment (excluding SHELFD_JWT_SECRET, DB DSNs, AI keys, etc.)
// while preserving system runtime paths, TLS CA bundle variables, and outbound proxy settings with localhost bypassed.
func buildCleanPluginEnv(scratchDir string) []string {
	allowKeys := []string{
		"PATH",
		"HOME",
		"TMPDIR",
		"TZ",
		"LANG",
		"LC_ALL",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
		"NODE_EXTRA_CA_CERTS",
		"HTTP_PROXY",
		"HTTPS_PROXY",
		"http_proxy",
		"https_proxy",
	}
	var env []string
	for _, k := range allowKeys {
		if v, ok := os.LookupEnv(k); ok && v != "" {
			env = append(env, k+"="+v)
		}
	}

	// Ensure loopback calls to 127.0.0.1 / localhost never traverse outbound HTTP proxies
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}
	for _, host := range []string{"127.0.0.1", "localhost"} {
		if !strings.Contains(noProxy, host) {
			if noProxy == "" {
				noProxy = host
			} else {
				noProxy += "," + host
			}
		}
	}
	env = append(env,
		"NO_PROXY="+noProxy,
		"no_proxy="+noProxy,
		"NODE_NO_WARNINGS=1",
		"SHELFD_DATA_DIR="+scratchDir,
	)
	return env
}

func hasRequiredConfigFields(schema map[string]any) bool {
	if schema == nil {
		return false
	}
	req, ok := schema["required"].([]any)
	if ok && len(req) > 0 {
		return true
	}
	reqStr, ok := schema["required"].([]string)
	return ok && len(reqStr) > 0
}

func checkRequiredFieldsPresent(schema, stored map[string]any) bool {
	if schema == nil {
		return true
	}
	var reqKeys []string
	if rawList, ok := schema["required"].([]any); ok {
		for _, item := range rawList {
			if s, ok := item.(string); ok {
				reqKeys = append(reqKeys, s)
			}
		}
	} else if strList, ok := schema["required"].([]string); ok {
		reqKeys = strList
	}
	for _, k := range reqKeys {
		v, exists := stored[k]
		if !exists || v == nil {
			return false
		}
		if s, ok := v.(string); ok && strings.TrimSpace(s) == "" {
			return false
		}
	}
	return true
}
