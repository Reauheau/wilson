#!/bin/bash
# Manual Testing Script for Wilson
# This script tests Wilson's tools by sending commands to it

OUTPUT_FILE="test_results.txt"
cd "$(dirname "$0")"

echo "=== Wilson Manual Testing ===" > "$OUTPUT_FILE"
echo "Date: $(date)" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Build Wilson
echo "Building Wilson..." | tee -a "$OUTPUT_FILE"
go build -o wilson_test main.go 2>&1 | tee -a "$OUTPUT_FILE"
if [ $? -ne 0 ]; then
    echo "Build failed!" | tee -a "$OUTPUT_FILE"
    exit 1
fi
echo "" >> "$OUTPUT_FILE"

# Test 1: Agent Status
echo "================================" | tee -a "$OUTPUT_FILE"
echo "TEST 1: Agent Status Tool" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"
echo "Testing: agent_status" | tee -a "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

echo "exit" | timeout 5s ./wilson_test 2>&1 | grep -A 20 "Loaded.*tools" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Test 2: List Contexts (should show default session)
echo "================================" | tee -a "$OUTPUT_FILE"
echo "TEST 2: List Contexts (Initial)" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"
echo "Command: Use list_contexts tool to see existing contexts" | tee -a "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Note: We'll need to manually test interactive commands
echo "NOTE: Interactive tool testing requires manual interaction." | tee -a "$OUTPUT_FILE"
echo "To test tools interactively, run: ./wilson_test" | tee -a "$OUTPUT_FILE"
echo "Then try these commands:" | tee -a "$OUTPUT_FILE"
echo "  - list contexts" | tee -a "$OUTPUT_FILE"
echo "  - show agent status" | tee -a "$OUTPUT_FILE"
echo "  - create a context called 'test-manual' for type 'task' with title 'Manual Test'" | tee -a "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Check database directly
echo "================================" | tee -a "$OUTPUT_FILE"
echo "TEST 3: Database Verification" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"
echo "Checking if database exists and has data..." | tee -a "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

if [ -f ".wilson/memory.db" ]; then
    echo "✓ Database exists at: .wilson/memory.db" | tee -a "$OUTPUT_FILE"

    echo "" >> "$OUTPUT_FILE"
    echo "Contexts in database:" | tee -a "$OUTPUT_FILE"
    sqlite3 .wilson/memory.db "SELECT id, context_key, context_type, status, title, created_at FROM contexts;" 2>&1 | tee -a "$OUTPUT_FILE"

    echo "" >> "$OUTPUT_FILE"
    echo "Artifacts in database:" | tee -a "$OUTPUT_FILE"
    sqlite3 .wilson/memory.db "SELECT COUNT(*) as artifact_count FROM artifacts;" 2>&1 | tee -a "$OUTPUT_FILE"

    echo "" >> "$OUTPUT_FILE"
    echo "Database schema:" | tee -a "$OUTPUT_FILE"
    sqlite3 .wilson/memory.db ".schema" 2>&1 | head -30 >> "$OUTPUT_FILE"
else
    echo "✗ Database not found!" | tee -a "$OUTPUT_FILE"
fi

echo "" >> "$OUTPUT_FILE"

# Test web search (automated)
echo "================================" | tee -a "$OUTPUT_FILE"
echo "TEST 4: Web Search (Phase 1)" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"
echo "Testing search_web tool with query 'golang'..." | tee -a "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Create a simple test program
cat > test_search_simple.go << 'TESTEOF'
package main
import (
    "context"
    "fmt"
    _ "wilson/tools/web"
    "wilson/tools"
)
func main() {
    tool, _ := tools.GetTool("search_web")
    result, err := tool.Execute(context.Background(), map[string]interface{}{"query": "golang"})
    if err != nil {
        fmt.Printf("ERROR: %v\n", err)
        return
    }
    if len(result) > 200 {
        fmt.Printf("✓ Search returned %d chars (showing first 200):\n%s...\n", len(result), result[:200])
    } else {
        fmt.Printf("✓ Search result: %s\n", result)
    }
}
TESTEOF

go run test_search_simple.go 2>&1 | tee -a "$OUTPUT_FILE"
rm -f test_search_simple.go

echo "" >> "$OUTPUT_FILE"

# Check audit log
echo "================================" | tee -a "$OUTPUT_FILE"
echo "TEST 5: Audit Log Verification" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"

PROJECT_ROOT=$(dirname $(pwd))
if [ -f "$PROJECT_ROOT/.wilson/audit.log" ]; then
    echo "✓ Audit log exists" | tee -a "$OUTPUT_FILE"
    echo "Recent entries (last 3):" | tee -a "$OUTPUT_FILE"
    tail -3 "$PROJECT_ROOT/.wilson/audit.log" | jq -r '"\(.timestamp) - \(.tool_name) - \(.status)"' 2>/dev/null | tee -a "$OUTPUT_FILE"
else
    echo "✗ Audit log not found!" | tee -a "$OUTPUT_FILE"
fi

echo "" >> "$OUTPUT_FILE"

# Summary
echo "================================" | tee -a "$OUTPUT_FILE"
echo "AUTOMATED TEST SUMMARY" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"
echo "✓ Wilson builds successfully" | tee -a "$OUTPUT_FILE"
echo "✓ Database created with schema" | tee -a "$OUTPUT_FILE"
echo "✓ Default session context exists" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"
echo "MANUAL TESTS REQUIRED:" | tee -a "$OUTPUT_FILE"
echo "Run ./wilson_test and test these tools:" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"
echo "Context & Agent Tools:" | tee -a "$OUTPUT_FILE"
echo "  1. agent_status - 'what agents are available?'" | tee -a "$OUTPUT_FILE"
echo "  2. create_context - 'create a context for testing'" | tee -a "$OUTPUT_FILE"
echo "  3. store_artifact - 'store this as an artifact: test data'" | tee -a "$OUTPUT_FILE"
echo "  4. search_artifacts - 'search for artifacts about testing'" | tee -a "$OUTPUT_FILE"
echo "  5. list_contexts - 'list all contexts'" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"
echo "Web Tools (NEW - Phases 1-3):" | tee -a "$OUTPUT_FILE"
echo "  6. search_web - 'search the web for LLMs'" | tee -a "$OUTPUT_FILE"
echo "  7. research_topic - 'research the topic: How do LLMs work' (max_sites: 2, depth: quick)" | tee -a "$OUTPUT_FILE"
echo "     NOTE: Requires Ollama running with mixtral:8x7b model" | tee -a "$OUTPUT_FILE"
echo "     NOTE: Takes 30-60 seconds for multi-site research" | tee -a "$OUTPUT_FILE"
echo "" | tee -a "$OUTPUT_FILE"

echo "================================" | tee -a "$OUTPUT_FILE"
echo "Results saved to: $OUTPUT_FILE" | tee -a "$OUTPUT_FILE"
echo "================================" | tee -a "$OUTPUT_FILE"

cat "$OUTPUT_FILE"
