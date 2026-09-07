# Security Sentinel Manual

This technical manual instructs the AI on auditing security controls, path traversal vulnerabilities, and authentication mechanisms in Shelfd.

## Invariants

1. **Zip Slip & Path Traversal Prevention**:
   * During EPUB unzipping, all archive entry filepaths must be validated against directory traversal:
     ```go
     destPath := filepath.Join(targetDir, entry.Name)
     if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(targetDir)+string(os.PathSeparator)) {
         return fmt.Errorf("illegal file path: %s", entry.Name)
     }
     ```
   * User-supplied author and title strings must be stripped of path separator characters before creating filesystem folders in `/library`.

2. **API Token Security**:
   * MCP API tokens must be generated using cryptographically secure random bytes (e.g. `crypto/rand`, 32 bytes hex-encoded).
   * Plaintext tokens must only be shown to the user once upon creation.
   * Store only the SHA-256 hash in the `api_tokens.token_hash` column.
   * Compare incoming Bearer tokens using `subtle.ConstantTimeCompare`.

3. **SSRF Guard on AI Endpoints**:
   * Validate configured `ai.base_url` values.
   * Reject cloud metadata endpoints (e.g., `169.254.169.254`) and loopback bypass attacks unless explicitly allowed in local dev mode.

4. **SQL Parameterization**:
   * Never construct SQL statements via string concatenation or `fmt.Sprintf`.
   * Use strictly parameterized queries with `:name` or `?` placeholders across all relational and `sqlite-vec` operations.
