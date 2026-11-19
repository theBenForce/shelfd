package plugins

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	// SecretSentinel is returned by GET /config in place of decrypted secret values.
	SecretSentinel = "__SHELFD_REDACTED__"
	encPrefix      = "enc:v1:"
)

// SecretFieldMeta describes secret and smart-extraction annotations on a JSON Schema property.
type SecretFieldMeta struct {
	IsSecret   bool
	CurlPaste  bool
	CookieName string
}

// DerivePluginKey derives a 32-byte AES-256 key from the server's jwt_secret scoped to pluginName.
func DerivePluginKey(masterSecret, pluginName string) ([]byte, error) {
	if strings.TrimSpace(masterSecret) == "" {
		return nil, errors.New("master secret cannot be empty")
	}
	if strings.TrimSpace(pluginName) == "" {
		return nil, errors.New("plugin name cannot be empty")
	}

	reader := hkdf.New(sha256.New, []byte(masterSecret), nil, []byte("shelfd-plugin-config:"+pluginName))
	key := make([]byte, 32)
	if _, err := io.ReadFull(reader, key); err != nil {
		return nil, fmt.Errorf("deriving plugin key: %w", err)
	}
	return key, nil
}

// EncryptValue encrypts plaintext with AES-256-GCM and encodes as "enc:v1:<base64(nonce||ciphertext)>".
func EncryptValue(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("initializing aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("initializing gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// DecryptValue decrypts an "enc:v1:..." string encrypted with EncryptValue.
func DecryptValue(key []byte, encoded string) (string, error) {
	if !strings.HasPrefix(encoded, encPrefix) {
		return encoded, nil
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, encPrefix))
	if err != nil {
		return "", fmt.Errorf("decoding encrypted secret: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("initializing aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("initializing gcm: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext shorter than nonce size")
	}
	nonce := raw[:gcm.NonceSize()]
	ciphertext := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypting secret: %w", err)
	}
	return string(plain), nil
}

// SecretFieldsFromSchema inspects a JSON Schema object's "properties" map for secret annotations.
func SecretFieldsFromSchema(schema map[string]any) map[string]SecretFieldMeta {
	res := make(map[string]SecretFieldMeta)
	if schema == nil {
		return res
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		return res
	}
	for field, rawProp := range props {
		propMap, ok := rawProp.(map[string]any)
		if !ok {
			continue
		}
		isSecret := false
		if b, ok := propMap["x-shelfd-secret"].(bool); ok && b {
			isSecret = true
		}
		if fmtStr, ok := propMap["format"].(string); ok && strings.EqualFold(fmtStr, "password") {
			isSecret = true
		}
		curlPaste, _ := propMap["x-shelfd-curl-paste"].(bool)
		cookieName, _ := propMap["x-shelfd-cookie-name"].(string)
		if isSecret || curlPaste {
			res[field] = SecretFieldMeta{
				IsSecret:   isSecret,
				CurlPaste:  curlPaste,
				CookieName: strings.TrimSpace(cookieName),
			}
		}
	}
	return res
}

// PrepareConfigForStorage normalizes incoming config values, applies Smart cURL/Cookie extraction,
// preserves unchanged redacted secrets, and encrypts secret fields for storage at rest.
func PrepareConfigForStorage(
	masterSecret, pluginName string,
	schema, incoming, existingStored map[string]any,
) (stored map[string]any, decrypted map[string]any, err error) {
	key, err := DerivePluginKey(masterSecret, pluginName)
	if err != nil {
		return nil, nil, err
	}
	secretMeta := SecretFieldsFromSchema(schema)

	stored = make(map[string]any)
	decrypted = make(map[string]any)

	for k, v := range existingStored {
		stored[k] = v
		if meta, ok := secretMeta[k]; ok && meta.IsSecret {
			if s, ok := v.(string); ok && s != "" {
				plain, dErr := DecryptValue(key, s)
				if dErr != nil {
					return nil, nil, fmt.Errorf("decrypting existing field %q: %w", k, dErr)
				}
				decrypted[k] = plain
			}
		} else {
			decrypted[k] = v
		}
	}

	for k, v := range incoming {
		meta, isSpecial := secretMeta[k]
		strVal, isStr := v.(string)

		if isSpecial && meta.IsSecret && isStr && strVal == SecretSentinel {
			// User submitted form without altering the existing masked secret
			continue
		}

		if isSpecial && meta.CurlPaste && isStr && strings.TrimSpace(strVal) != "" {
			strVal = ExtractCredentialFromPaste(strVal, meta.CookieName)
			v = strVal
		}

		if isSpecial && meta.IsSecret {
			if !isStr || strings.TrimSpace(strVal) == "" {
				delete(stored, k)
				delete(decrypted, k)
				continue
			}
			enc, encErr := EncryptValue(key, strVal)
			if encErr != nil {
				return nil, nil, fmt.Errorf("encrypting field %q: %w", k, encErr)
			}
			stored[k] = enc
			decrypted[k] = strVal
		} else {
			stored[k] = v
			decrypted[k] = v
		}
	}

	return stored, decrypted, nil
}

// DecryptStoredConfig returns the decrypted config map for delivery to a plugin over RPC.
func DecryptStoredConfig(
	masterSecret, pluginName string,
	schema, stored map[string]any,
) (decrypted map[string]any, secretFieldsSet []string, err error) {
	key, err := DerivePluginKey(masterSecret, pluginName)
	if err != nil {
		return nil, nil, err
	}
	secretMeta := SecretFieldsFromSchema(schema)
	decrypted = make(map[string]any, len(stored))

	for k, v := range stored {
		if meta, ok := secretMeta[k]; ok && meta.IsSecret {
			if s, ok := v.(string); ok && s != "" {
				plain, dErr := DecryptValue(key, s)
				if dErr != nil {
					return nil, nil, fmt.Errorf("decrypting field %q: %w", k, dErr)
				}
				decrypted[k] = plain
				secretFieldsSet = append(secretFieldsSet, k)
			}
		} else {
			decrypted[k] = v
		}
	}
	sort.Strings(secretFieldsSet)
	return decrypted, secretFieldsSet, nil
}

// RedactStoredConfig masks encrypted secret fields with SecretSentinel for GET /api/v1/plugins/{name}/config.
func RedactStoredConfig(schema, stored map[string]any) (redacted map[string]any, secretFieldsSet []string) {
	secretMeta := SecretFieldsFromSchema(schema)
	redacted = make(map[string]any, len(stored))

	for k, v := range stored {
		if meta, ok := secretMeta[k]; ok && meta.IsSecret {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				redacted[k] = SecretSentinel
				secretFieldsSet = append(secretFieldsSet, k)
			}
		} else {
			redacted[k] = v
		}
	}
	sort.Strings(secretFieldsSet)
	return redacted, secretFieldsSet
}

var (
	curlQuotedArgRe   = regexp.MustCompile(`(?:-H|--header|-b|--cookie)\s+(?:'([^']*)'|"((?:\\.|[^"\\])*)"|([^\s]+))`)
	rawBearerHeaderRe = regexp.MustCompile(`(?i)^(?:authorization\s*:\s*)?bearer\s+(.+)$`)
	rawCookiePrefixRe = regexp.MustCompile(`(?i)^cookie\s*:\s*(.+)$`)
)

// ExtractCredentialFromPaste extracts a target cookie, Bearer token, or raw secret from a pasted
// DevTools "Copy as cURL" command, HTTP header line, or raw token string.
func ExtractCredentialFromPaste(input string, targetCookieName string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}

	// Normalize escaped line continuations from multiline cURL commands
	flat := strings.ReplaceAll(trimmed, "\\\r\n", " ")
	flat = strings.ReplaceAll(flat, "\\\n", " ")

	if strings.HasPrefix(strings.ToLower(flat), "curl ") {
		var authVal, cookieVal string
		for _, m := range curlQuotedArgRe.FindAllStringSubmatch(flat, -1) {
			flagPart := strings.ToLower(strings.Fields(m[0])[0])
			arg := m[1]
			if arg == "" && m[2] != "" {
				arg = strings.ReplaceAll(m[2], `\"`, `"`)
			} else if arg == "" {
				arg = m[3]
			}
			arg = strings.TrimSpace(arg)
			if flagPart == "-b" || flagPart == "--cookie" {
				cookieVal = arg
				continue
			}
			lowerArg := strings.ToLower(arg)
			if strings.HasPrefix(lowerArg, "cookie:") {
				cookieVal = strings.TrimSpace(arg[len("cookie:"):])
			} else if strings.HasPrefix(lowerArg, "authorization:") {
				rawAuth := strings.TrimSpace(arg[len("authorization:"):])
				if strings.HasPrefix(strings.ToLower(rawAuth), "bearer ") {
					rawAuth = strings.TrimSpace(rawAuth[len("bearer "):])
				}
				authVal = rawAuth
			}
		}

		if targetCookieName != "" && cookieVal != "" {
			if val := extractNamedCookie(cookieVal, targetCookieName); val != "" {
				return val
			}
		}
		if authVal != "" {
			return authVal
		}
		if cookieVal != "" {
			return cookieVal
		}
	}

	if m := rawBearerHeaderRe.FindStringSubmatch(trimmed); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}

	cookieLine := trimmed
	if m := rawCookiePrefixRe.FindStringSubmatch(trimmed); len(m) == 2 {
		cookieLine = strings.TrimSpace(m[1])
	}

	if targetCookieName != "" && (strings.Contains(cookieLine, "=") || strings.Contains(cookieLine, ";")) {
		if val := extractNamedCookie(cookieLine, targetCookieName); val != "" {
			return val
		}
	}

	return trimmed
}

func extractNamedCookie(cookieHeader, targetName string) string {
	parts := strings.Split(cookieHeader, ";")
	for _, p := range parts {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 && kv[0] == targetName {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}
