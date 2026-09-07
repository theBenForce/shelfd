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

echo "=== All entrypoint tests passed successfully ==="
