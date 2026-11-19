#!/bin/sh
set -e

echo "=== Running docker/entrypoint_test.sh ==="

# 1. Verify syntax of entrypoint.sh with sh -n
sh -n docker/entrypoint.sh
echo "[PASS] entrypoint.sh syntax is valid"

# 2. Test non-root pass-through
OUTPUT=$(PUID=1001 PGID=1001 UMASK=022 ./docker/entrypoint.sh echo "hello-shelfd")
if [ "$OUTPUT" != "hello-shelfd" ]; then
    echo "[FAIL] Expected 'hello-shelfd', got '$OUTPUT'"
    exit 1
fi
echo "[PASS] entrypoint passes through command when running non-root"

# 3. Test that entrypoint does not reference recursive chown on /library
if grep -q "chown.*-R.*/library" docker/entrypoint.sh; then
    echo "[FAIL] Entrypoint contains recursive chown on /library! Audiobookshelf invariant violated!"
    exit 1
fi
echo "[PASS] Audiobookshelf invariant preserved: zero recursive chown on /library"

# 4. Test built-in plugin seeding and upgrade refresh while preserving .data
TMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TMP_ROOT"' EXIT

BUILTIN_DIR="$TMP_ROOT/builtin-plugins"
DATA_DIR="$TMP_ROOT/data"
mkdir -p "$BUILTIN_DIR/project-gutenberg/dist" "$BUILTIN_DIR/yoto/dist" "$BUILTIN_DIR/notebooklm/dist" "$DATA_DIR/plugins"

for pname in project-gutenberg yoto notebooklm; do
    printf '{"name":"%s","version":"0.1.0"}\n' "$pname" > "$BUILTIN_DIR/$pname/plugin.json"
    printf 'export const version = "0.1.0";\n' > "$BUILTIN_DIR/$pname/dist/index.mjs"
done

# Extract and run the seeding block from docker/entrypoint.sh with our temp paths
SEED_SCRIPT="$TMP_ROOT/seed_block.sh"
sed -n '/if \[ -d "\/usr\/share\/shelfd\/builtin-plugins" \]; then/,/chown -R/p' docker/entrypoint.sh \
    | grep -v 'chown -R' \
    | sed "s|/usr/share/shelfd/builtin-plugins|$BUILTIN_DIR|g" \
    | sed "s|/data/plugins|$DATA_DIR/plugins|g" > "$SEED_SCRIPT"

sh "$SEED_SCRIPT"

for pname in project-gutenberg yoto notebooklm; do
    if [ ! -f "$DATA_DIR/plugins/$pname/plugin.json" ] || [ ! -f "$DATA_DIR/plugins/$pname/dist/index.mjs" ]; then
        echo "[FAIL] Built-in plugin '$pname' was not seeded into $DATA_DIR/plugins/$pname"
        exit 1
    fi
done
echo "[PASS] Built-in plugins project-gutenberg, yoto, and notebooklm seeded on first boot"

# Create .data scratch file inside yoto and upgrade built-in version to 0.2.0
mkdir -p "$DATA_DIR/plugins/yoto/.data"
printf 'persisted-scratch-state\n' > "$DATA_DIR/plugins/yoto/.data/cache.txt"
printf '{"name":"yoto","version":"0.2.0"}\n' > "$BUILTIN_DIR/yoto/plugin.json"
printf 'export const version = "0.2.0";\n' > "$BUILTIN_DIR/yoto/dist/index.mjs"

sh "$SEED_SCRIPT"

if ! grep -q '"0.2.0"' "$DATA_DIR/plugins/yoto/plugin.json"; then
    echo "[FAIL] Existing yoto plugin.json was not refreshed on upgrade"
    exit 1
fi
if ! grep -q '"0.2.0"' "$DATA_DIR/plugins/yoto/dist/index.mjs"; then
    echo "[FAIL] Existing yoto dist/index.mjs was not refreshed on upgrade"
    exit 1
fi
if [ "$(cat "$DATA_DIR/plugins/yoto/.data/cache.txt")" != "persisted-scratch-state" ]; then
    echo "[FAIL] Existing yoto .data directory was not preserved during upgrade refresh"
    exit 1
fi
echo "[PASS] Built-in plugin upgrade refreshes plugin.json and dist/ while preserving .data"

echo "=== All entrypoint tests passed successfully ==="
