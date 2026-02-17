#!/usr/bin/env bash
set -euo pipefail

BINARY="./bin/token-eval"
TMPFILE=$(mktemp /tmp/token-eval-test.XXXXXX)
DB="${TMPFILE}.db"
rm -f "$TMPFILE"
trap "rm -f $DB" EXIT

pass=0
fail=0

check() {
    local name="$1"
    shift
    if "$@" >/dev/null 2>&1; then
        echo "  ✅ $name"
        pass=$((pass + 1))
    else
        echo "  ❌ $name"
        fail=$((fail + 1))
    fi
}

check_output() {
    local name="$1"
    local expected="$2"
    shift 2
    local out
    out=$("$@" 2>&1) || true
    if echo "$out" | grep -q "$expected"; then
        echo "  ✅ $name"
        pass=$((pass + 1))
    else
        echo "  ❌ $name (expected '$expected' in output)"
        echo "     got: $out"
        fail=$((fail + 1))
    fi
}

echo "🧪 token-eval acceptance tests"
echo "   DB: $DB"
echo ""

# Record command
echo "📝 record command"
check "basic record" $BINARY --db "$DB" record -p test-project -m claude-sonnet-4 -i 1000 -o 500 --intent "test the record command"
check "record with result" $BINARY --db "$DB" record -p test-project -m gpt-4o -i 500 -o 200 --result pass -q 90 --task "code-review"
check "record with cache" $BINARY --db "$DB" record -p other-project -m claude-opus-4 -i 2000 -o 1000 --cache-create 500 --cache-read 3000

# Stdin JSON
echo '{"task":"stdin-task","prompt":"hello from stdin","intent":"test stdin","input_tokens":100,"output_tokens":50}' | \
    $BINARY --db "$DB" record -p test-project -m claude-sonnet-4 > /tmp/te-stdin-out.json 2>&1
check_output "stdin JSON record" "stdin-task" cat /tmp/te-stdin-out.json

echo ""

# Query command
echo "🔍 query command"
check_output "query all" "test-project" $BINARY --db "$DB" query
check_output "query by project" "test-project" $BINARY --db "$DB" query -p test-project
check_output "query by model" "gpt-4o" $BINARY --db "$DB" query -m gpt-4o
check_output "query by result" "pass" $BINARY --db "$DB" query --result pass
check_output "query with --full" "hello from stdin" $BINARY --db "$DB" query -p test-project --full
check_output "query with limit" "test-project" $BINARY --db "$DB" query --limit 1

# FTS search
check_output "FTS search" "stdin-task" $BINARY --db "$DB" query "hello from stdin"

echo ""

# Price commands
echo "💰 price commands"
check_output "price list" "claude-sonnet-4" $BINARY --db "$DB" price list
check "price set" $BINARY --db "$DB" price set custom-model --input 5.0 --output 20.0 --provider custom
check_output "price list after set" "custom-model" $BINARY --db "$DB" price list
check "price rm" $BINARY --db "$DB" price rm custom-model

echo ""

# Provider auto-detection
echo "🔍 provider detection"
OUT=$($BINARY --db "$DB" record -p test-project -m deepseek-r1 -i 100 -o 50 2>&1)
check_output "auto-detect deepseek" "deepseek" echo "$OUT"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━"
echo "Results: $pass passed, $fail failed"

if [ "$fail" -gt 0 ]; then
    exit 1
fi
echo "🎉 All acceptance tests passed!"
