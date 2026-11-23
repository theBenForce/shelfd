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

// SecretFieldsFromSchema inspects a JSON Schema for secret annotations. Object properties are
// scanned recursively: properties of array "items" produce index-free paths such as
// "libraries[*].cookie", so repeatable object-array cards can carry per-item secrets.
func SecretFieldsFromSchema(schema map[string]any) map[string]SecretFieldMeta {
	res := make(map[string]SecretFieldMeta)
	if schema == nil {
		return res
	}
	props, _ := schema["properties"].(map[string]any)
	collectSecretSpecs(props, "", res)
	return res
}

func collectSecretSpecs(props map[string]any, prefix string, out map[string]SecretFieldMeta) {
	for name, rawProp := range props {
		propMap, ok := rawProp.(map[string]any)
		if !ok {
			continue
		}
		addSecretSpecIfAnnotated(propMap, prefix+name, out)
		if items, ok := propMap["items"].(map[string]any); ok {
			itemProps, hasItemProps := items["properties"].(map[string]any)
			if hasItemProps {
				collectSecretSpecs(itemProps, prefix+name+"[*].", out)
			}
		}
	}
}

func addSecretSpecIfAnnotated(propMap map[string]any, path string, out map[string]SecretFieldMeta) {
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
		out[path] = SecretFieldMeta{
			IsSecret:   isSecret,
			CurlPaste:  curlPaste,
			CookieName: strings.TrimSpace(cookieName),
		}
	}
}

// PrepareConfigForStorage normalizes incoming config values, applies Smart cURL/Cookie extraction,
// preserves unchanged redacted secrets, and encrypts secret fields for storage at rest. Object
// values are walked recursively; array-of-object properties (repeatable cards) get per-element
// handling with sentinel preservation matched by position.
func PrepareConfigForStorage(
	masterSecret, pluginName string,
	schema, incoming, existingStored map[string]any,
) (stored map[string]any, decrypted map[string]any, err error) {
	key, err := DerivePluginKey(masterSecret, pluginName)
	if err != nil {
		return nil, nil, err
	}
	stored = make(map[string]any)
	decrypted = make(map[string]any)
	props, _ := schema["properties"].(map[string]any)
	if pErr := prepareObjectNode(props, incoming, existingStored, key, stored, decrypted); pErr != nil {
		return nil, nil, pErr
	}
	return stored, decrypted, nil
}

// seedExistingValue seeds an existing value into stored/decrypted, decrypting annotated secrets.
func seedExistingValue(storedM, decM map[string]any, k string, v any, meta SecretFieldMeta, hasMeta bool, key []byte) error {
	storedM[k] = v
	if hasMeta && meta.IsSecret {
		if s, ok := v.(string); ok && s != "" {
			plain, dErr := DecryptValue(key, s)
			if dErr != nil {
				return fmt.Errorf("decrypting existing field %q: %w", k, dErr)
			}
			decM[k] = plain
		}
	} else {
		decM[k] = v
	}
	return nil
}

// prepareIncomingField applies one incoming value over seeded existing values, implementing
// sentinel preservation, Smart cURL/Cookie extraction, and at-rest encryption.
func prepareIncomingField(storedM, decM map[string]any, k string, v any, meta SecretFieldMeta, hasMeta bool, key []byte) error {
	strVal, isStr := v.(string)

	if hasMeta && meta.IsSecret && isStr && strVal == SecretSentinel {
		// User submitted form without altering the existing masked secret
		return nil
	}

	if hasMeta && meta.CurlPaste && isStr && strings.TrimSpace(strVal) != "" {
		v = ExtractCredentialFromPaste(strVal, meta.CookieName)
		strVal = v.(string)
	}

	if hasMeta && meta.IsSecret {
		if !isStr || strings.TrimSpace(strVal) == "" {
			delete(storedM, k)
			delete(decM, k)
			return nil
		}
		enc, encErr := EncryptValue(key, strVal)
		if encErr != nil {
			return fmt.Errorf("encrypting field %q: %w", k, encErr)
		}
		storedM[k] = enc
		decM[k] = strVal
		return nil
	}

	storedM[k] = v
	decM[k] = v
	return nil
}

// itemObjectSchema returns the items schema when a property declares an array of objects.
func itemObjectSchema(propMap map[string]any) (map[string]any, bool) {
	items, ok := propMap["items"].(map[string]any)
	if !ok {
		return nil, false
	}
	itemProps, hasProps := items["properties"].(map[string]any)
	if !hasProps {
		return nil, false
	}
	return itemProps, true
}

// secretMetaForProp extracts secret annotations declared on a single property schema node.
func secretMetaForProp(propMap map[string]any) (SecretFieldMeta, bool) {
	if propMap == nil {
		return SecretFieldMeta{}, false
	}
	var meta SecretFieldMeta
	hasMeta := false
	if b, ok := propMap["x-shelfd-secret"].(bool); ok && b {
		meta.IsSecret = true
		hasMeta = true
	}
	if f, ok := propMap["format"].(string); ok && strings.EqualFold(f, "password") {
		meta.IsSecret = true
		hasMeta = true
	}
	if cp, ok := propMap["x-shelfd-curl-paste"].(bool); ok && cp {
		meta.CurlPaste = true
		hasMeta = true
	}
	if cn, ok := propMap["x-shelfd-cookie-name"].(string); ok {
		meta.CookieName = strings.TrimSpace(cn)
	}
	return meta, hasMeta
}

func asAnySlice(v any) []any {
	s, _ := v.([]any)
	return s
}

// prepareObjectNode seeds existing values then applies incoming overrides for one object node.
func prepareObjectNode(
	nodeProps map[string]any,
	incomingObj, existingObj any,
	key []byte,
	storedOut map[string]any,
	decryptedOut map[string]any,
) error {
	inM, _ := incomingObj.(map[string]any)
	exM, _ := existingObj.(map[string]any)

	keys := make(map[string]bool, len(nodeProps)+len(inM)+len(exM))
	for k := range nodeProps {
		keys[k] = true
	}
	for k := range exM {
		keys[k] = true
	}
	for k := range inM {
		keys[k] = true
	}

	for k := range keys {
		exV, hasEx := exM[k]
		inV, hasIn := inM[k]
		var propMap map[string]any
		var meta SecretFieldMeta
		hasMeta := false
		if raw, ok := nodeProps[k].(map[string]any); ok {
			propMap = raw
			meta, hasMeta = secretMetaForProp(propMap)
		}

		if itemProps, isItemObj := itemObjectSchema(propMap); isItemObj && (asAnySlice(inV) != nil || asAnySlice(exV) != nil) {
			storedArr, decArr, aErr := prepareObjectArray(itemProps, inV, exV, key)
			if aErr != nil {
				return fmt.Errorf("field %q: %w", k, aErr)
			}
			storedOut[k] = storedArr
			decryptedOut[k] = decArr
			continue
		}

		if hasEx {
			if err := seedExistingValue(storedOut, decryptedOut, k, exV, meta, hasMeta, key); err != nil {
				return err
			}
		}
		if hasIn {
			if err := prepareIncomingField(storedOut, decryptedOut, k, inV, meta, hasMeta, key); err != nil {
				return err
			}
		}
	}
	return nil
}

// prepareObjectArray processes array-of-object values, matching each incoming element to its
// existing counterpart by a stable non-secret identity field when possible (so reordered or
// shrunken arrays keep per-element secrets bound to the correct record), falling back to
// positional matching for elements without a shared identity.
func prepareObjectArray(itemProps map[string]any, inV, exV any, key []byte) ([]any, []any, error) {
	inArr := asAnySlice(inV)
	exArr := asAnySlice(exV)
	n := len(inArr)
	if n == 0 && inArr == nil {
		n = len(exArr)
	}
	storedArr := make([]any, 0, n)
	decArr := make([]any, 0, n)
	exUsed := make(map[int]bool, len(exArr))
	for i := range n {
		var inEl, exEl any
		if i < len(inArr) {
			inEl = inArr[i]
		}
		switch j := matchExistingIndex(inEl, exArr, itemProps, exUsed); {
		case j >= 0:
			exEl = exArr[j]
			exUsed[j] = true
		case i < len(exArr) && !exUsed[i]:
			exEl = exArr[i]
			exUsed[i] = true
		}
		storedEl := map[string]any{}
		decEl := map[string]any{}
		if err := prepareObjectNode(itemProps, inEl, exEl, key, storedEl, decEl); err != nil {
			return nil, nil, err
		}
		storedArr = append(storedArr, storedEl)
		decArr = append(decArr, decEl)
	}
	return storedArr, decArr, nil
}

// matchExistingIndex finds the existing element that is the same record as inEl by a stable
// non-secret property whose value is present and equal on both sides (e.g. a library
// subdomain). Returns -1 when no identity match exists; callers fall back to position.
func matchExistingIndex(inEl any, exArr []any, itemProps map[string]any, used map[int]bool) int {
	inM, ok := inEl.(map[string]any)
	if !ok || len(exArr) == 0 {
		return -1
	}
	for k, v := range inM {
		sIn, isStr := v.(string)
		if !isStr || strings.TrimSpace(sIn) == "" {
			continue
		}
		if propMap, isProp := itemProps[k].(map[string]any); isProp {
			if meta, hasMeta := secretMetaForProp(propMap); hasMeta && meta.IsSecret {
				continue // secrets must not identify records
			}
		}
		for j := range exArr {
			if used[j] {
				continue
			}
			exM, isExM := exArr[j].(map[string]any)
			if !isExM {
				continue
			}
			if sEx, isStrEx := exM[k].(string); isStrEx && sEx == sIn {
				return j
			}
		}
	}
	return -1
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
	props, _ := schema["properties"].(map[string]any)
	decrypted = make(map[string]any, len(stored))
	seenPaths := map[string]bool{}
	if dErr := decryptObjectNode(props, stored, key, "", decrypted, seenPaths); dErr != nil {
		return nil, nil, dErr
	}
	set := make([]string, 0, len(seenPaths))
	for p := range seenPaths {
		set = append(set, p)
	}
	sort.Strings(set)
	return decrypted, set, nil
}

// decryptObjectNode walks stored values under one object node, decrypting annotated secrets.
func decryptObjectNode(nodeProps map[string]any, storedObj any, key []byte, pathPrefix string, decryptedOut map[string]any, seenPaths map[string]bool) error {
	storedM, _ := storedObj.(map[string]any)
	for k, v := range storedM {
		var propMap map[string]any
		meta := SecretFieldMeta{}
		if raw, ok := nodeProps[k].(map[string]any); ok {
			propMap = raw
			meta, _ = secretMetaForProp(propMap)
		}
		if itemProps, isItemObj := itemObjectSchema(propMap); isItemObj && asAnySlice(v) != nil {
			arr := asAnySlice(v)
			outArr := make([]any, 0, len(arr))
			for _, el := range arr {
				elMap := map[string]any{}
				if dErr := decryptObjectNode(itemProps, el, key, pathPrefix+k+"[*].", elMap, seenPaths); dErr != nil {
					return dErr
				}
				outArr = append(outArr, elMap)
			}
			decryptedOut[k] = outArr
			continue
		}
		if meta.IsSecret {
			if s, ok := v.(string); ok && s != "" {
				plain, dErr := DecryptValue(key, s)
				if dErr != nil {
					return fmt.Errorf("decrypting field %q: %w", pathPrefix+k, dErr)
				}
				decryptedOut[k] = plain
				seenPaths[pathPrefix+k] = true
				continue
			}
		}
		decryptedOut[k] = v
	}
	return nil
}

// redactObjectNode masks annotated secrets under one object node with SecretSentinel.
func redactObjectNode(nodeProps map[string]any, storedObj any, pathPrefix string, redactedOut map[string]any, seenPaths map[string]bool) {
	storedM, _ := storedObj.(map[string]any)
	for k, v := range storedM {
		var propMap map[string]any
		meta := SecretFieldMeta{}
		if raw, ok := nodeProps[k].(map[string]any); ok {
			propMap = raw
			meta, _ = secretMetaForProp(propMap)
		}
		if itemProps, isItemObj := itemObjectSchema(propMap); isItemObj && asAnySlice(v) != nil {
			arr := asAnySlice(v)
			outArr := make([]any, 0, len(arr))
			for _, el := range arr {
				elMap := map[string]any{}
				redactObjectNode(itemProps, el, pathPrefix+k+"[*].", elMap, seenPaths)
				outArr = append(outArr, elMap)
			}
			redactedOut[k] = outArr
			continue
		}
		if meta.IsSecret {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				redactedOut[k] = SecretSentinel
				seenPaths[pathPrefix+k] = true
				continue
			}
		}
		redactedOut[k] = v
	}
}

// RedactStoredConfig masks encrypted secret fields with SecretSentinel for GET /api/v1/plugins/{name}/config.
func RedactStoredConfig(schema, stored map[string]any) (redacted map[string]any, secretFieldsSet []string) {
	props, _ := schema["properties"].(map[string]any)
	redacted = make(map[string]any, len(stored))
	seenPaths := map[string]bool{}
	redactObjectNode(props, stored, "", redacted, seenPaths)
	set := make([]string, 0, len(seenPaths))
	for p := range seenPaths {
		set = append(set, p)
	}
	sort.Strings(set)
	return redacted, set
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
