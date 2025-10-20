// Test the compile tool functionality
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"wilson/core/registry"
	_ "wilson/capabilities/code_intelligence/ast"
	_ "wilson/capabilities/code_intelligence/build"
)

func main() {
	fmt.Println("=== Testing compile Tool ===\n")

	ctx := context.Background()

	// Get the compile tool
	compileTool, err := tools.GetTool("compile")
	if err != nil {
		fmt.Printf("❌ compile tool not found: %v\n", err)
		return
	}

	fmt.Println("Test 1: Compile current directory (should succeed)")
	result, err := compileTool.Execute(ctx, map[string]interface{}{})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Parse result
	var compileResult map[string]interface{}
	json.Unmarshal([]byte(result), &compileResult)

	fmt.Printf("Success: %v\n", compileResult["success"])
	fmt.Printf("Duration: %v ms\n", compileResult["duration_ms"])
	fmt.Printf("Error count: %v\n", compileResult["error_count"])
	fmt.Printf("Message: %s\n", compileResult["message"])

	if errors, ok := compileResult["errors"].([]interface{}); ok && len(errors) > 0 {
		fmt.Println("\nCompilation Errors:")
		for i, err := range errors {
			if errMap, ok := err.(map[string]interface{}); ok {
				fmt.Printf("  %d. %s:%v:%v: [%s] %s\n",
					i+1,
					errMap["file"],
					errMap["line"],
					errMap["column"],
					errMap["type"],
					errMap["message"])
			}
		}
	}

	fmt.Println("\n✅ compile tool test complete!")
	fmt.Println("\n=== Testing run_tests Tool ===\n")

	// Get the run_tests tool
	testTool, err := tools.GetTool("run_tests")
	if err != nil {
		fmt.Printf("❌ run_tests tool not found: %v\n", err)
		return
	}

	fmt.Println("Test 2: Run agent package tests")
	result, err = testTool.Execute(ctx, map[string]interface{}{
		"path":    "agent",
		"verbose": false,
	})
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err)
		return
	}

	// Parse result
	var testResult map[string]interface{}
	json.Unmarshal([]byte(result), &testResult)

	fmt.Printf("Success: %v\n", testResult["success"])
	fmt.Printf("Duration: %v ms\n", testResult["duration_ms"])
	fmt.Printf("Tests run: %v\n", testResult["tests_run"])
	fmt.Printf("Tests passed: %v\n", testResult["tests_passed"])
	fmt.Printf("Tests failed: %v\n", testResult["tests_failed"])
	fmt.Printf("Message: %s\n", testResult["message"])

	if failures, ok := testResult["failed_tests"].([]interface{}); ok && len(failures) > 0 {
		fmt.Println("\nTest Failures:")
		for i, fail := range failures {
			if failMap, ok := fail.(map[string]interface{}); ok {
				fmt.Printf("  %d. %s in %s\n", i+1, failMap["test_name"], failMap["package"])
				if output, ok := failMap["output"].(string); ok && output != "" {
					fmt.Printf("     Output: %s\n", output)
				}
			}
		}
	}

	fmt.Println("\n✅ run_tests tool test complete!")
}
