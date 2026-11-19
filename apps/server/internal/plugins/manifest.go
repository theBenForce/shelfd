package plugins

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	pluginNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,32}$`)
	semverRe     = regexp.MustCompile(`^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
)

// EntrypointSpec declares the child process command and arguments (no shell interpretation).
type EntrypointSpec struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// RuntimeSpec declares optional runtime expectations for documentation and operator diagnostics.
type RuntimeSpec struct {
	Family     string `json:"family,omitempty"`
	MinVersion string `json:"min_version,omitempty"`
}

// PermissionsSpec declares advisory permissions disclosed to operators in the UI and logs.
type PermissionsSpec struct {
	NetworkDomains []string `json:"network_domains,omitempty"`
	LibraryRead    bool     `json:"library_read,omitempty"`
	LibraryWrite   bool     `json:"library_write,omitempty"`
}

// ToolDef describes an MCP tool exposed by a plugin.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

// CommandDef describes a REST/UI command exposed by a plugin.
type CommandDef struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

// CatalogDescriptor describes optional catalog browsing/searching metadata declared by a plugin.
type CatalogDescriptor struct {
	Modes            []string `json:"modes,omitempty"`             // "browse", "search"
	SupportedFormats []string `json:"supported_formats,omitempty"` // e.g. ["epub", "m4b"]
	SearchHint       string   `json:"search_hint,omitempty"`
	RequiresAuth     bool     `json:"requires_auth,omitempty"`
}

// Manifest represents the parsed plugin.json file inside /data/plugins/<name>/plugin.json.
type Manifest struct {
	Name             string             `json:"name"`
	Version          string             `json:"version"`
	DisplayName      string             `json:"display_name,omitempty"`
	Description      string             `json:"description,omitempty"`
	Homepage         string             `json:"homepage,omitempty"`
	Entrypoint       EntrypointSpec     `json:"entrypoint"`
	Runtime          *RuntimeSpec       `json:"runtime,omitempty"`
	Daemon           bool               `json:"daemon,omitempty"`
	Capabilities     []string           `json:"capabilities,omitempty"`      // "tools", "commands", "events", "catalog"
	CatalogModes     []string           `json:"catalog_modes,omitempty"`     // "browse", "search"
	SupportedFormats []string           `json:"supported_formats,omitempty"` // e.g. ["epub", "m4b"]
	APIScopes        []string           `json:"api_scopes,omitempty"`        // "uploads:stage", "uploads:write", "books:read"
	Permissions      PermissionsSpec    `json:"permissions,omitempty"`
	ConfigSchema     map[string]any     `json:"config_schema,omitempty"`
	Tools            []ToolDef          `json:"tools,omitempty"`
	Commands         []CommandDef       `json:"commands,omitempty"`
	Catalog          *CatalogDescriptor `json:"catalog,omitempty"`
}

// ResolvedDisplayName returns DisplayName or falls back to Name.
func (m *Manifest) ResolvedDisplayName() string {
	if strings.TrimSpace(m.DisplayName) != "" {
		return strings.TrimSpace(m.DisplayName)
	}
	return m.Name
}

// HasCapability checks if the manifest declares a given capability.
func (m *Manifest) HasCapability(capName string) bool {
	for _, c := range m.Capabilities {
		if strings.EqualFold(strings.TrimSpace(c), capName) {
			return true
		}
	}
	return false
}

// ParseManifest parses and validates raw plugin.json bytes.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing plugin.json: %w", err)
	}
	if err := ValidateManifest(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadManifest reads and validates plugin.json from a plugin directory.
func LoadManifest(pluginDir string) (*Manifest, error) {
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", manifestPath, err)
	}
	m, err := ParseManifest(data)
	if err != nil {
		return nil, err
	}
	dirBase := filepath.Base(filepath.Clean(pluginDir))
	if dirBase != m.Name {
		return nil, fmt.Errorf("plugin directory %q must match manifest name %q", dirBase, m.Name)
	}
	return m, nil
}

// ValidateManifest validates plugin identity, semver, entrypoint, capabilities, and scopes.
func ValidateManifest(m *Manifest) error {
	if m == nil {
		return errors.New("manifest is nil")
	}
	m.Name = strings.TrimSpace(m.Name)
	if !pluginNameRe.MatchString(m.Name) {
		return fmt.Errorf("invalid plugin name %q: must match ^[a-z][a-z0-9_-]{1,32}$", m.Name)
	}
	m.Version = strings.TrimSpace(m.Version)
	if !semverRe.MatchString(m.Version) {
		return fmt.Errorf("invalid plugin version %q: must be valid semver", m.Version)
	}
	m.Entrypoint.Command = strings.TrimSpace(m.Entrypoint.Command)
	if m.Entrypoint.Command == "" {
		return errors.New("entrypoint.command is required")
	}
	if strings.Contains(m.Entrypoint.Command, "..") {
		return errors.New("entrypoint.command must not contain '..'")
	}

	validCaps := map[string]bool{
		"tools":    true,
		"commands": true,
		"events":   true,
		"catalog":  true,
	}
	for _, c := range m.Capabilities {
		if !validCaps[strings.ToLower(strings.TrimSpace(c))] {
			return fmt.Errorf("unsupported capability %q", c)
		}
	}

	validScopes := map[string]bool{
		"uploads:stage": true,
		"uploads:write": true,
		"books:read":    true,
	}
	for _, s := range m.APIScopes {
		if !validScopes[strings.ToLower(strings.TrimSpace(s))] {
			return fmt.Errorf("unsupported api_scope %q", s)
		}
	}

	return nil
}

const maxPluginArchiveBytes = 50 << 20 // 50 MB limit for extracted plugin bundle

// ExtractPluginZip safely validates and extracts a .zip plugin archive into pluginsDir/<manifest.name>.
// Enforces Zip-Slip traversal prevention and validates plugin.json before replacing an existing directory.
func ExtractPluginZip(archiveBytes []byte, pluginsDir string) (*Manifest, error) {
	if len(archiveBytes) == 0 {
		return nil, errors.New("empty plugin archive")
	}
	if len(archiveBytes) > maxPluginArchiveBytes {
		return nil, errors.New("plugin archive exceeds 50MB limit")
	}

	zr, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		return nil, fmt.Errorf("invalid zip archive: %w", err)
	}

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("creating plugins directory: %w", err)
	}

	// Determine whether archive files share a single top-level directory or sit at root
	prefix := detectCommonZipRoot(zr.File)

	var manifestBytes []byte
	for _, f := range zr.File {
		relName := strings.TrimPrefix(filepath.ToSlash(f.Name), prefix)
		relName = strings.TrimPrefix(relName, "/")
		if relName == "plugin.json" && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("opening plugin.json in archive: %w", err)
			}
			manifestBytes, err = io.ReadAll(io.LimitReader(rc, 1<<20))
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("reading plugin.json in archive: %w", err)
			}
			break
		}
	}
	if len(manifestBytes) == 0 {
		return nil, errors.New("archive does not contain a root plugin.json")
	}

	manifest, err := ParseManifest(manifestBytes)
	if err != nil {
		return nil, err
	}

	stagingDir, err := os.MkdirTemp(pluginsDir, ".install-"+manifest.Name+"-*")
	if err != nil {
		return nil, fmt.Errorf("creating staging directory: %w", err)
	}
	cleanupStaging := true
	defer func() {
		if cleanupStaging {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	var totalExtracted int64
	cleanTarget := filepath.Clean(stagingDir) + string(os.PathSeparator)

	for _, f := range zr.File {
		relName := strings.TrimPrefix(filepath.ToSlash(f.Name), prefix)
		relName = strings.TrimPrefix(relName, "/")
		if relName == "" {
			continue
		}

		destPath := filepath.Join(stagingDir, filepath.FromSlash(relName))
		if !strings.HasPrefix(filepath.Clean(destPath), cleanTarget) {
			return nil, fmt.Errorf("illegal archive file path: %s", f.Name)
		}

		if f.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("symlinks are not permitted in plugin archives: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return nil, fmt.Errorf("creating directory %s: %w", relName, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("creating parent directory for %s: %w", relName, err)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening archive entry %s: %w", relName, err)
		}

		mode := os.FileMode(0644)
		if f.Mode()&0111 != 0 {
			mode = 0755
		}

		outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
		if err != nil {
			rc.Close()
			return nil, fmt.Errorf("creating file %s: %w", relName, err)
		}

		written, copyErr := io.Copy(outFile, io.LimitReader(rc, maxPluginArchiveBytes-totalExtracted+1))
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			return nil, fmt.Errorf("extracting %s: %w", relName, copyErr)
		}
		totalExtracted += written
		if totalExtracted > maxPluginArchiveBytes {
			return nil, errors.New("extracted plugin size exceeds 50MB limit")
		}
	}

	finalDir := filepath.Join(pluginsDir, manifest.Name)
	// Preserve existing data scratch subdirectory if updating an installed plugin
	existingScratch := filepath.Join(finalDir, ".data")
	stagedScratch := filepath.Join(stagingDir, ".data")
	if info, err := os.Stat(existingScratch); err == nil && info.IsDir() {
		_ = os.Rename(existingScratch, stagedScratch)
	}

	_ = os.RemoveAll(finalDir)
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return nil, fmt.Errorf("activating plugin directory: %w", err)
	}
	cleanupStaging = false

	return manifest, nil
}

func detectCommonZipRoot(files []*zip.File) string {
	for _, f := range files {
		clean := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if clean == "plugin.json" {
			return ""
		}
	}
	var candidate string
	for _, f := range files {
		clean := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		if clean == "" || strings.HasPrefix(clean, "__MACOSX/") {
			continue
		}
		parts := strings.SplitN(clean, "/", 2)
		if len(parts) < 2 {
			return ""
		}
		if candidate == "" {
			candidate = parts[0] + "/"
		} else if parts[0]+"/" != candidate {
			return ""
		}
	}
	return candidate
}
