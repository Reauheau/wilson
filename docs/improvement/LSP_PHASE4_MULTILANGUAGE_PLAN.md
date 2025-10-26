# LSP Phase 4: Multi-Language Support Implementation Plan

**Date:** October 26, 2025
**Status:** Planning Phase
**Priority:** HIGH - Expands Wilson to Python, JavaScript/TypeScript, Rust
**Estimated Effort:** 5-7 days
**Prerequisite:** Phase 1 (Go) complete ✅

---

## 🎯 Executive Summary

Extend Wilson's LSP integration from Go-only to supporting **4 major languages**:
- ✅ **Go** (gopls) - Already implemented in Phase 1
- 🆕 **Python** (Pylance/pyright or pylsp)
- 🆕 **JavaScript/TypeScript** (typescript-language-server)
- 🆕 **Rust** (rust-analyzer)

**Impact:** Wilson becomes a **universal coding assistant**, not just a Go tool
**Key Challenge:** Each language server has different setup, configuration, and quirks

---

## 📚 Background Research

### Current State (Phase 1 - Go Only)

**What Works:**
- `go/lsp/manager.go` - Multi-language architecture already designed
- `go/lsp/client.go` - Language detection stub at line 122-140
- Language server executables already mapped at line 572-601
- All 5 LSP tools working for Go: `get_diagnostics`, `go_to_definition`, `find_references`, `get_symbols`, `get_hover_info`

**Architecture:**
```
Manager (multi-language) → Client (language-specific) → Language Server Process
```

**Key Finding:** The architecture is **already multi-language ready**! We just need to:
1. Configure each language server correctly
2. Handle language-specific quirks
3. Test extensively
4. Update prompts and documentation

---

## 🗺️ Language Server Research

### 1. Python Language Servers

**Option A: Pylance/Pyright** (Microsoft - RECOMMENDED)
- **Executable:** `pyright-langserver`
- **Installation:** `npm install -g pyright`
- **Pros:**
  - Fast, actively maintained
  - Excellent type checking
  - Great diagnostics
  - Used by VS Code
- **Cons:**
  - Node.js dependency
  - Requires proper Python environment detection
- **Configuration:**
  ```json
  {
    "python.analysis.typeCheckingMode": "basic",
    "python.pythonPath": "/usr/bin/python3"
  }
  ```

**Option B: Python LSP Server (pylsp)**
- **Executable:** `pylsp`
- **Installation:** `pip install python-lsp-server[all]`
- **Pros:**
  - Pure Python
  - Lightweight
  - No Node.js needed
- **Cons:**
  - Slower than Pyright
  - Less comprehensive type checking
- **Configuration:** Minimal, works out of box

**DECISION:** Use **Pyright** as primary, fall back to pylsp if pyright not available

---

### 2. JavaScript/TypeScript Language Server

**typescript-language-server** (Official LSP wrapper for tsserver)
- **Executable:** `typescript-language-server`
- **Installation:** `npm install -g typescript-language-server typescript`
- **Language ID:** `javascript`, `typescript`, `javascriptreact`, `typescriptreact`
- **Args:** `--stdio`
- **Configuration:**
  ```json
  {
    "format": {
      "indentSize": 2
    },
    "javascript": {
      "suggest": {
        "autoImports": true
      }
    }
  }
  ```
- **Quirks:**
  - Needs `tsconfig.json` or `jsconfig.json` for best results
  - Auto-creates config if missing
  - Supports both .js and .ts files
  - React JSX requires `typescriptreact` language ID

**Alternative: Deno LSP** (for Deno projects)
- Built-in to Deno: `deno lsp`
- Use if project has `deno.json`

---

### 3. Rust Language Server

**rust-analyzer** (Official Rust LSP)
- **Executable:** `rust-analyzer`
- **Installation:**
  ```bash
  # Via rustup (recommended)
  rustup component add rust-analyzer

  # Or standalone
  brew install rust-analyzer  # macOS
  pacman -S rust-analyzer     # Arch Linux
  ```
- **Language ID:** `rust`
- **Args:** None (stdio mode default)
- **Configuration:**
  ```json
  {
    "rust-analyzer.checkOnSave.command": "clippy",
    "rust-analyzer.cargo.features": "all"
  }
  ```
- **Requirements:**
  - Must be in a Cargo project (needs `Cargo.toml`)
  - Runs `cargo check` in background for diagnostics
  - First run is slow (builds project metadata)

**Quirks:**
- Very resource-intensive (high CPU/memory)
- Needs Cargo.toml in workspace root
- Takes 5-30 seconds to initialize (first time)
- Incremental after that (~500ms)

---

## 🏗️ Implementation Architecture

### Phase 4A: Core Multi-Language Support (Days 1-2)

#### 1. Enhance Language Detection

**File:** `go/lsp/manager.go` (lines 122-140)

**Current:** Basic extension mapping
```go
func detectLanguage(filePath string) string {
    switch {
    case strings.HasSuffix(filePath, ".go"):
        return "go"
    case strings.HasSuffix(filePath, ".py"):
        return "python"
    // ... etc
    }
}
```

**Enhanced:** Multi-extension + shebang detection
```go
// detectLanguage returns the language and language ID based on file
func detectLanguage(filePath string) (language string, languageID string) {
    ext := filepath.Ext(filePath)
    base := filepath.Base(filePath)

    // Check specific files first
    switch base {
    case "Cargo.toml", "Cargo.lock":
        return "rust", "toml"
    case "package.json":
        return "javascript", "json"
    case "pyproject.toml":
        return "python", "toml"
    }

    // Extension-based detection
    switch ext {
    case ".go":
        return "go", "go"
    case ".py", ".pyi":
        return "python", "python"
    case ".js":
        return "javascript", "javascript"
    case ".mjs":
        return "javascript", "javascript"
    case ".cjs":
        return "javascript", "javascript"
    case ".jsx":
        return "javascript", "javascriptreact"
    case ".ts":
        return "typescript", "typescript"
    case ".tsx":
        return "typescript", "typescriptreact"
    case ".rs":
        return "rust", "rust"
    default:
        // Try shebang for scripts without extension
        if ext == "" {
            if lang := detectShebang(filePath); lang != "" {
                return lang, lang
            }
        }
        return "", ""
    }
}

// detectShebang reads first line to detect interpreter
func detectShebang(filePath string) string {
    f, err := os.Open(filePath)
    if err != nil {
        return ""
    }
    defer f.Close()

    scanner := bufio.NewScanner(f)
    if scanner.Scan() {
        firstLine := scanner.Text()
        if strings.HasPrefix(firstLine, "#!") {
            if strings.Contains(firstLine, "python") {
                return "python"
            }
            if strings.Contains(firstLine, "node") || strings.Contains(firstLine, "bun") {
                return "javascript"
            }
        }
    }
    return ""
}
```

**Why:** Handles edge cases like .jsx, .tsx, shebangs, config files

---

#### 2. Update Language Server Configuration

**File:** `go/lsp/client.go` (lines 572-601)

**Current:** Hardcoded executables
```go
func getLanguageServerExecutable(language string) string {
    switch language {
    case "go":
        return "gopls"
    case "python":
        return "pyright-langserver"
    // ...
    }
}
```

**Enhanced:** Fallback chain + validation
```go
// Language server configuration
type ServerConfig struct {
    Primary   string   // Primary executable to try
    Fallbacks []string // Fallback executables
    Args      []string // Command-line arguments
}

var languageServers = map[string]ServerConfig{
    "go": {
        Primary:   "gopls",
        Fallbacks: []string{},
        Args:      []string{},
    },
    "python": {
        Primary:   "pyright-langserver",
        Fallbacks: []string{"pylsp"},
        Args:      []string{"--stdio"},
    },
    "javascript": {
        Primary:   "typescript-language-server",
        Fallbacks: []string{},
        Args:      []string{"--stdio"},
    },
    "typescript": {
        Primary:   "typescript-language-server",
        Fallbacks: []string{},
        Args:      []string{"--stdio"},
    },
    "rust": {
        Primary:   "rust-analyzer",
        Fallbacks: []string{},
        Args:      []string{},
    },
}

// getLanguageServerExecutable finds the first available language server
func getLanguageServerExecutable(language string) string {
    config, ok := languageServers[language]
    if !ok {
        return ""
    }

    // Try primary first
    if execPath, err := exec.LookPath(config.Primary); err == nil {
        return execPath
    }

    // Try fallbacks
    for _, fallback := range config.Fallbacks {
        if execPath, err := exec.LookPath(fallback); err == nil {
            fmt.Printf("[LSP] Using fallback: %s (primary %s not found)\n", fallback, config.Primary)
            return execPath
        }
    }

    return ""
}

// getLanguageServerArgs returns command-line arguments
func getLanguageServerArgs(language string) []string {
    if config, ok := languageServers[language]; ok {
        return config.Args
    }
    return []string{}
}

// ValidateLanguageServer checks if language server is available
func ValidateLanguageServer(language string) error {
    executable := getLanguageServerExecutable(language)
    if executable == "" {
        config := languageServers[language]
        return fmt.Errorf("language server for %s not found (tried: %s, %v)",
            language, config.Primary, config.Fallbacks)
    }
    return nil
}
```

**Why:** Graceful degradation, helpful error messages, easy to add new servers

---

#### 3. Add Language-Specific Initialization

**File:** `go/lsp/client.go` (line 140-179 - Initialize method)

**Enhancement:** Language-specific init params

```go
// Initialize sends the initialize request with language-specific params
func (c *Client) Initialize(ctx context.Context, rootURI string) error {
    if c.initialized {
        return nil
    }

    c.rootURI = rootURI

    // Build base initialization params
    params := InitializeParams{
        ProcessID: -1,
        RootURI:   rootURI,
        Capabilities: c.getClientCapabilities(),
        InitializationOptions: c.getInitializationOptions(),
    }

    // Send initialize request
    result, err := c.SendRequest(ctx, "initialize", params)
    if err != nil {
        return fmt.Errorf("initialize failed: %w", err)
    }

    // Parse server capabilities
    var initResult InitializeResult
    if err := json.Unmarshal(result, &initResult); err != nil {
        return fmt.Errorf("failed to parse initialize result: %w", err)
    }

    // Store server capabilities for later reference
    c.serverCapabilities = initResult.Capabilities

    // Send initialized notification
    if err := c.sendNotification("initialized", struct{}{}); err != nil {
        return fmt.Errorf("failed to send initialized notification: %w", err)
    }

    c.initialized = true
    return nil
}

// getClientCapabilities returns language-specific client capabilities
func (c *Client) getClientCapabilities() ClientCapabilities {
    base := ClientCapabilities{
        TextDocument: TextDocumentClientCapabilities{
            PublishDiagnostics: PublishDiagnosticsClientCapabilities{
                RelatedInformation: true,
                TagSupport:         &DiagnosticTagSupport{ValueSet: []DiagnosticTag{1, 2}},
                VersionSupport:     true,
            },
            Synchronization: &TextDocumentSyncClientCapabilities{
                DynamicRegistration: false,
                WillSave:            false,
                WillSaveWaitUntil:   false,
                DidSave:             true,
            },
            Completion: &CompletionClientCapabilities{
                DynamicRegistration: false,
                CompletionItem: &CompletionItemClientCapabilities{
                    SnippetSupport: false,
                },
            },
            Hover: &HoverClientCapabilities{
                ContentFormat: []MarkupKind{"markdown", "plaintext"},
            },
            SignatureHelp: &SignatureHelpClientCapabilities{
                DynamicRegistration: false,
            },
            Definition: &DefinitionClientCapabilities{
                DynamicRegistration: false,
                LinkSupport:         false,
            },
            References: &ReferenceClientCapabilities{
                DynamicRegistration: false,
            },
            DocumentSymbol: &DocumentSymbolClientCapabilities{
                DynamicRegistration: false,
                HierarchicalDocumentSymbolSupport: true,
            },
        },
        Workspace: &WorkspaceClientCapabilities{
            ApplyEdit:              true,
            WorkspaceEdit:          &WorkspaceEditClientCapabilities{},
            DidChangeConfiguration: &DidChangeConfigurationClientCapabilities{},
            DidChangeWatchedFiles:  &DidChangeWatchedFilesClientCapabilities{},
        },
    }

    return base
}

// getInitializationOptions returns language-specific initialization options
func (c *Client) getInitializationOptions() interface{} {
    switch c.language {
    case "python":
        return map[string]interface{}{
            "python": map[string]interface{}{
                "analysis": map[string]interface{}{
                    "typeCheckingMode": "basic",
                    "diagnosticMode":   "openFilesOnly",
                    "autoSearchPaths":  true,
                },
            },
        }
    case "javascript", "typescript":
        return map[string]interface{}{
            "preferences": map[string]interface{}{
                "includeInlayParameterNameHints": "all",
                "includeInlayFunctionParameterTypeHints": true,
            },
        }
    case "rust":
        return map[string]interface{}{
            "checkOnSave": map[string]interface{}{
                "command": "clippy",
            },
            "cargo": map[string]interface{}{
                "features": "all",
            },
        }
    default:
        return nil
    }
}
```

**Why:** Each language server expects different init options for optimal behavior

---

### Phase 4B: Testing Infrastructure (Day 3)

#### 1. Language Server Installation Script

**File:** `scripts/install_language_servers.sh` (NEW)

```bash
#!/bin/bash
# Installs all language servers for Wilson

set -e

echo "🔧 Installing Language Servers for Wilson"
echo ""

# Check prerequisites
command -v npm >/dev/null 2>&1 || { echo "❌ npm not found. Install Node.js first."; exit 1; }
command -v pip3 >/dev/null 2>&1 || { echo "⚠️  pip3 not found. Python support will be limited."; }
command -v cargo >/dev/null 2>&1 || { echo "⚠️  cargo not found. Rust support will be limited."; }

# Go (gopls)
echo "📦 Installing gopls (Go language server)..."
if command -v go >/dev/null 2>&1; then
    go install golang.org/x/tools/gopls@latest
    echo "✅ gopls installed"
else
    echo "❌ Go not found, skipping gopls"
fi

# Python (Pyright)
echo "📦 Installing pyright (Python language server)..."
npm install -g pyright
echo "✅ pyright installed"

echo "📦 Installing pylsp (Python LSP fallback)..."
if command -v pip3 >/dev/null 2>&1; then
    pip3 install python-lsp-server[all]
    echo "✅ pylsp installed"
fi

# JavaScript/TypeScript
echo "📦 Installing typescript-language-server..."
npm install -g typescript-language-server typescript
echo "✅ typescript-language-server installed"

# Rust (rust-analyzer)
echo "📦 Installing rust-analyzer..."
if command -v rustup >/dev/null 2>&1; then
    rustup component add rust-analyzer
    echo "✅ rust-analyzer installed via rustup"
elif command -v cargo >/dev/null 2>&1; then
    cargo install rust-analyzer
    echo "✅ rust-analyzer installed via cargo"
else
    echo "⚠️  Rust not found, skipping rust-analyzer"
fi

echo ""
echo "✅ Language server installation complete!"
echo ""
echo "Installed servers:"
command -v gopls >/dev/null 2>&1 && echo "  ✅ gopls (Go)" || echo "  ❌ gopls (Go)"
command -v pyright-langserver >/dev/null 2>&1 && echo "  ✅ pyright (Python)" || echo "  ❌ pyright (Python)"
command -v pylsp >/dev/null 2>&1 && echo "  ✅ pylsp (Python fallback)" || echo "  ⚠️  pylsp (Python fallback)"
command -v typescript-language-server >/dev/null 2>&1 && echo "  ✅ typescript-language-server (JS/TS)" || echo "  ❌ typescript-language-server (JS/TS)"
command -v rust-analyzer >/dev/null 2>&1 && echo "  ✅ rust-analyzer (Rust)" || echo "  ❌ rust-analyzer (Rust)"
echo ""
echo "Run 'wilson check-lsp' to verify all language servers work correctly."
```

**Make executable:**
```bash
chmod +x scripts/install_language_servers.sh
```

---

#### 2. Language Server Health Check Command

**File:** `go/cmd/check_lsp.go` (NEW)

```go
package main

import (
    "context"
    "fmt"
    "os"
    "time"

    "wilson/lsp"
)

// CheckLSP verifies all language servers are installed and working
func CheckLSP() {
    fmt.Println("🔍 Wilson LSP Health Check")
    fmt.Println("")

    languages := []string{"go", "python", "javascript", "typescript", "rust"}
    manager := lsp.NewManager()

    for _, lang := range languages {
        fmt.Printf("Testing %s language server... ", lang)

        // Check if executable exists
        if err := lsp.ValidateLanguageServer(lang); err != nil {
            fmt.Printf("❌ NOT INSTALLED\n")
            fmt.Printf("   Error: %v\n", err)
            continue
        }

        // Try to start and initialize
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        client, err := manager.GetClient(ctx, lang)
        if err != nil {
            fmt.Printf("❌ FAILED TO START\n")
            fmt.Printf("   Error: %v\n", err)
            continue
        }

        // Check if running
        if !client.IsRunning() {
            fmt.Printf("❌ NOT RUNNING\n")
            continue
        }

        fmt.Printf("✅ OK\n")
    }

    fmt.Println("")
    fmt.Println("Run 'scripts/install_language_servers.sh' to install missing servers.")
}

func main() {
    if len(os.Args) > 1 && os.Args[1] == "check-lsp" {
        CheckLSP()
        return
    }
    // ... rest of main
}
```

---

#### 3. Multi-Language Test Suite

**File:** `go/tests/lsp_multilang_test.go` (NEW)

```go
package tests

import (
    "context"
    "os"
    "path/filepath"
    "testing"
    "time"

    "wilson/lsp"
)

// TestMultiLanguageDetection tests language detection
func TestMultiLanguageDetection(t *testing.T) {
    tests := []struct {
        filename string
        wantLang string
        wantID   string
    }{
        {"main.go", "go", "go"},
        {"script.py", "python", "python"},
        {"app.js", "javascript", "javascript"},
        {"component.jsx", "javascript", "javascriptreact"},
        {"main.ts", "typescript", "typescript"},
        {"Component.tsx", "typescript", "typescriptreact"},
        {"lib.rs", "rust", "rust"},
    }

    for _, tt := range tests {
        t.Run(tt.filename, func(t *testing.T) {
            lang, langID := detectLanguage(tt.filename)
            if lang != tt.wantLang {
                t.Errorf("language = %q, want %q", lang, tt.wantLang)
            }
            if langID != tt.wantID {
                t.Errorf("languageID = %q, want %q", langID, tt.wantID)
            }
        })
    }
}

// TestPythonLSP tests Python language server
func TestPythonLSP(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping LSP test in short mode")
    }

    // Check if Python LSP is installed
    if err := lsp.ValidateLanguageServer("python"); err != nil {
        t.Skip("Python language server not installed:", err)
    }

    manager := lsp.NewManager()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create test Python file
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.py")
    err := os.WriteFile(testFile, []byte(`
def greet(name: str) -> str:
    return f"Hello, {name}!"

# Error: undefined variable
print(undefined_var)
`), 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Get client
    client, err := manager.GetClientForFile(ctx, testFile)
    if err != nil {
        t.Fatal("Failed to get Python client:", err)
    }

    // Open document
    content, _ := os.ReadFile(testFile)
    fileURI := "file://" + testFile
    err = client.OpenDocument(ctx, fileURI, "python", string(content))
    if err != nil {
        t.Fatal("Failed to open document:", err)
    }

    // Wait for diagnostics
    time.Sleep(2 * time.Second)

    // Get diagnostics
    diagnostics := client.GetDiagnostics(fileURI)

    // Should have error for undefined_var
    if len(diagnostics) == 0 {
        t.Error("Expected diagnostics for undefined_var, got none")
    }

    // Test go-to-definition on greet function
    locations, err := client.GoToDefinition(ctx, fileURI, 1, 4) // Line 1, "def"
    if err != nil {
        t.Error("GoToDefinition failed:", err)
    }
    if len(locations) == 0 {
        t.Error("Expected definition location, got none")
    }

    t.Logf("✅ Python LSP working: %d diagnostics, %d definition locations",
        len(diagnostics), len(locations))
}

// TestJavaScriptLSP tests JavaScript/TypeScript language server
func TestJavaScriptLSP(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping LSP test in short mode")
    }

    if err := lsp.ValidateLanguageServer("javascript"); err != nil {
        t.Skip("JavaScript language server not installed:", err)
    }

    manager := lsp.NewManager()
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create test JavaScript file
    tmpDir := t.TempDir()
    testFile := filepath.Join(tmpDir, "test.js")
    err := os.WriteFile(testFile, []byte(`
function add(a, b) {
    return a + b;
}

// Error: undefined variable
console.log(undefinedVar);
`), 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Get client
    client, err := manager.GetClientForFile(ctx, testFile)
    if err != nil {
        t.Fatal("Failed to get JavaScript client:", err)
    }

    // Open document
    content, _ := os.ReadFile(testFile)
    fileURI := "file://" + testFile
    err = client.OpenDocument(ctx, fileURI, "javascript", string(content))
    if err != nil {
        t.Fatal("Failed to open document:", err)
    }

    // Wait for diagnostics
    time.Sleep(2 * time.Second)

    // Get diagnostics
    diagnostics := client.GetDiagnostics(fileURI)

    t.Logf("✅ JavaScript LSP working: %d diagnostics", len(diagnostics))
}

// TestRustLSP tests Rust language server
func TestRustLSP(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping LSP test in short mode")
    }

    if err := lsp.ValidateLanguageServer("rust"); err != nil {
        t.Skip("Rust language server not installed:", err)
    }

    manager := lsp.NewManager()
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // Rust is slower
    defer cancel()

    // Create test Rust project
    tmpDir := t.TempDir()

    // Create Cargo.toml
    cargoToml := filepath.Join(tmpDir, "Cargo.toml")
    err := os.WriteFile(cargoToml, []byte(`
[package]
name = "test"
version = "0.1.0"
edition = "2021"
`), 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Create src/main.rs
    srcDir := filepath.Join(tmpDir, "src")
    os.Mkdir(srcDir, 0755)
    testFile := filepath.Join(srcDir, "main.rs")
    err = os.WriteFile(testFile, []byte(`
fn main() {
    println!("Hello, world!");
    // Error: undefined variable
    let x = undefined_var;
}
`), 0644)
    if err != nil {
        t.Fatal(err)
    }

    // Get client (root should be project dir with Cargo.toml)
    client, err := manager.GetClient(ctx, "rust")
    if err != nil {
        t.Fatal("Failed to get Rust client:", err)
    }

    // Initialize with project root
    if err := client.Initialize(ctx, "file://"+tmpDir); err != nil {
        t.Fatal("Failed to initialize:", err)
    }

    // Open document
    content, _ := os.ReadFile(testFile)
    fileURI := "file://" + testFile
    err = client.OpenDocument(ctx, fileURI, "rust", string(content))
    if err != nil {
        t.Fatal("Failed to open document:", err)
    }

    // Wait longer for Rust (builds project metadata)
    time.Sleep(10 * time.Second)

    // Get diagnostics
    diagnostics := client.GetDiagnostics(fileURI)

    t.Logf("✅ Rust LSP working: %d diagnostics", len(diagnostics))
}
```

---

### Phase 4C: Agent Integration (Day 4)

#### 1. Update Agent Prompts for Multi-Language

**File:** `go/agent/agents/code_agent.go` (buildSystemPrompt method)

**Add after line 452 (LSP Best Practices section):**

```go
**Multi-Language Support:**
→ Wilson now supports Go, Python, JavaScript/TypeScript, and Rust
→ LSP tools work automatically for all supported languages
→ Use get_diagnostics after EVERY code change regardless of language
→ Language detection is automatic based on file extension

**Language-Specific Notes:**

**Python:**
- Language server: Pyright or pylsp
- Use get_diagnostics for type checking
- Supports .py files and .pyi stubs

**JavaScript/TypeScript:**
- Language server: typescript-language-server
- Supports .js, .jsx, .ts, .tsx files
- Auto-detects React components (.jsx/.tsx)
- Diagnostics include type errors and JSDoc validation

**Rust:**
- Language server: rust-analyzer
- Requires Cargo.toml in project root
- First diagnostics check may be slow (5-30s)
- Incremental checks are fast (~500ms)
- Clippy lints enabled by default
```

---

#### 2. Update LSP Tool Descriptions

**File:** `go/capabilities/code_intelligence/lsp_diagnostics.go`

**Update Description at line 27:**

```go
Description: "Get real-time diagnostics (errors, warnings, hints) from language server. Supports Go, Python, JavaScript, TypeScript, Rust. Use after every code change to catch issues immediately.",
```

**Similar updates for all 5 LSP tools:** `lsp_goto_definition.go`, `lsp_find_references.go`, `lsp_get_symbols.go`, `lsp_hover.go`

---

### Phase 4D: Documentation & Examples (Day 5)

#### 1. Multi-Language Usage Guide

**File:** `docs/LSP_MULTILANGUAGE_GUIDE.md` (NEW)

```markdown
# Wilson Multi-Language LSP Guide

Wilson supports code intelligence for 4 languages via LSP:

## Supported Languages

| Language | Server | Extensions | Status |
|----------|--------|-----------|--------|
| Go | gopls | .go | ✅ Stable |
| Python | Pyright/pylsp | .py, .pyi | ✅ Stable |
| JavaScript | typescript-language-server | .js, .jsx, .mjs, .cjs | ✅ Stable |
| TypeScript | typescript-language-server | .ts, .tsx | ✅ Stable |
| Rust | rust-analyzer | .rs | ✅ Stable |

## Installation

Run the installation script:
```bash
./scripts/install_language_servers.sh
```

Verify installation:
```bash
wilson check-lsp
```

## Usage Examples

### Python Example

```bash
wilson "Create a Python web scraper that fetches news articles in ~/python_scraper"
```

Wilson will:
1. Generate Python code with proper typing
2. Call get_diagnostics automatically
3. Fix any type errors before presenting code
4. Use Pyright for real-time feedback

### JavaScript/React Example

```bash
wilson "Create a React counter component in ~/react-app/Counter.jsx"
```

Wilson will:
1. Detect .jsx extension → use javascriptreact language ID
2. Generate React component code
3. Check diagnostics with typescript-language-server
4. Fix JSX syntax errors
5. Ensure proper import statements

### Rust Example

```bash
wilson "Create a CLI tool in Rust that parses JSON in ~/rust_cli"
```

Wilson will:
1. Initialize Cargo project (Cargo.toml + src/)
2. Generate Rust code
3. Wait for rust-analyzer to build metadata
4. Check diagnostics (may take 5-30s first time)
5. Fix any compilation errors with Clippy suggestions

## Language Server Configuration

### Python (Pyright)

Location: `~/.config/wilson/lsp/python.json`

```json
{
  "python.analysis.typeCheckingMode": "basic",
  "python.analysis.diagnosticMode": "openFilesOnly",
  "python.pythonPath": "/usr/bin/python3"
}
```

### TypeScript

Location: `~/.config/wilson/lsp/typescript.json`

```json
{
  "preferences": {
    "includeInlayParameterNameHints": "all"
  },
  "javascript.suggest.autoImports": true,
  "typescript.suggest.autoImports": true
}
```

### Rust

Location: `~/.config/wilson/lsp/rust.json`

```json
{
  "checkOnSave": {
    "command": "clippy"
  },
  "cargo": {
    "features": "all"
  }
}
```

## Troubleshooting

### Python: "pyright-langserver not found"
```bash
npm install -g pyright
```

### JavaScript: Diagnostics not showing
Ensure project has `tsconfig.json` or `jsconfig.json`:
```bash
wilson "Create tsconfig.json in my project"
```

### Rust: Very slow first check
This is normal. rust-analyzer builds project metadata on first run.
Subsequent checks are fast (<1s).

### General: Language server crashed
Restart the server:
```bash
wilson "restart lsp for python"
```

Or restart all:
```bash
wilson "restart all lsp servers"
```
```

---

#### 2. Test Data for Multi-Language Testing

**Directory:** `go/tests/testdata/multilang/` (NEW)

Create test files for each language:

**Python:** `tests/testdata/multilang/sample.py`
```python
from typing import List, Optional

def process_items(items: List[str], filter_empty: bool = True) -> List[str]:
    """Process a list of items."""
    result = []
    for item in items:
        if not filter_empty or item:
            result.append(item.upper())
    return result

# Test with errors
print(undefined_variable)  # Error: undefined
x: int = "string"          # Error: type mismatch
```

**JavaScript:** `tests/testdata/multilang/sample.js`
```javascript
/**
 * @param {string[]} items
 * @returns {string[]}
 */
function processItems(items) {
    return items.map(item => item.toUpperCase());
}

// Test with errors
console.log(undefinedVariable);  // Error: undefined
const x = 5;
x = 10;  // Error: const reassignment
```

**Rust:** `tests/testdata/multilang/sample.rs`
```rust
fn process_items(items: Vec<String>) -> Vec<String> {
    items.iter()
        .map(|item| item.to_uppercase())
        .collect()
}

fn main() {
    let items = vec!["hello".to_string(), "world".to_string()];
    println!("{:?}", process_items(items));

    // Test with errors
    let x = undefined_var;  // Error: undefined
}
```

---

### Phase 4E: Performance Optimization (Day 6)

#### 1. Lazy Language Server Initialization

**Problem:** Starting all 5 language servers on Wilson startup wastes resources

**Solution:** Start servers on-demand (already implemented in manager.go line 27-64)

**Verify:**
```go
// Manager.GetClient already does lazy init:
func (m *Manager) GetClient(ctx context.Context, language string) (*Client, error) {
    // Check if client already exists and running
    m.mu.RLock()
    client, exists := m.clients[language]
    m.mu.RUnlock()

    if exists && client.IsRunning() {
        return client, nil  // ✅ Reuse existing
    }

    // Create new client only if needed
    // ...
}
```

**Result:** Go server starts on first .go file, Python on first .py file, etc.

---

#### 2. Language Server Lifecycle Management

**Add to manager.go:**

```go
// StopIdleClients stops language servers that haven't been used in N minutes
func (m *Manager) StopIdleClients(idleTimeout time.Duration) {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    for lang, client := range m.clients {
        if now.Sub(client.lastUsed) > idleTimeout {
            fmt.Printf("[LSP] Stopping idle %s server\n", lang)
            _ = client.Stop()
            delete(m.clients, lang)
        }
    }
}

// Add to Client struct:
type Client struct {
    // ... existing fields ...
    lastUsed time.Time  // NEW: Track last request time
}

// Update SendRequest to track usage:
func (c *Client) SendRequest(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
    c.lastUsed = time.Now()  // NEW: Update timestamp
    // ... rest of method ...
}
```

**Background goroutine in main.go:**
```go
// Start idle client cleanup
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        lspManager.StopIdleClients(10 * time.Minute)
    }
}()
```

---

#### 3. Resource Monitoring

**Add metrics to manager.go:**

```go
// Stats returns statistics about running language servers
func (m *Manager) Stats() map[string]interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()

    stats := map[string]interface{}{
        "active_servers": len(m.clients),
        "servers":        make(map[string]map[string]interface{}),
    }

    for lang, client := range m.clients {
        stats["servers"].(map[string]map[string]interface{})[lang] = map[string]interface{}{
            "running":     client.IsRunning(),
            "initialized": client.initialized,
            "last_used":   client.lastUsed.Format(time.RFC3339),
            "uptime":      time.Since(client.startTime).String(),
        }
    }

    return stats
}
```

**CLI command:**
```bash
wilson lsp-stats
```

Output:
```
LSP Server Statistics:
  Active servers: 3

  go (gopls):
    Status: Running
    Uptime: 5m23s
    Last used: 2s ago

  python (pyright):
    Status: Running
    Uptime: 2m10s
    Last used: 45s ago

  rust (rust-analyzer):
    Status: Running
    Uptime: 1m05s
    Last used: 1m ago
```

---

### Phase 4F: Edge Cases & Error Handling (Day 7)

#### 1. Language Server Crash Recovery

**Add to client.go:**

```go
// listen method enhancement for crash detection
func (c *Client) listen() {
    scanner := newLSPScanner(c.stdout)

    for c.running.Load() {
        msg, err := scanner.ReadMessage()
        if err != nil {
            if err != io.EOF {
                fmt.Printf("[LSP %s] Read error: %v\n", c.language, err)
            }

            // Check if process crashed
            if c.cmd.ProcessState != nil && c.cmd.ProcessState.Exited() {
                fmt.Printf("[LSP %s] Server crashed with exit code: %d\n",
                    c.language, c.cmd.ProcessState.ExitCode())

                // Mark as crashed
                c.running.Store(false)
                c.crashed = true
            }
            break
        }

        // ... rest of listen logic ...
    }
}

// Auto-restart on next request
func (m *Manager) GetClient(ctx context.Context, language string) (*Client, error) {
    m.mu.RLock()
    client, exists := m.clients[language]
    m.mu.RUnlock()

    if exists {
        // Check if crashed
        if client.crashed {
            fmt.Printf("[LSP] Auto-restarting crashed %s server\n", language)
            _ = m.StopClient(language)
            // Will create new client below
        } else if client.IsRunning() {
            return client, nil
        }
    }

    // Create new client...
}
```

---

#### 2. Mixed-Language Projects

**Problem:** Project with multiple languages (e.g., Go backend + React frontend)

**Solution:** Detect workspace root, start appropriate servers per file

**File:** `go/lsp/workspace.go` (NEW)

```go
package lsp

import (
    "os"
    "path/filepath"
)

// DetectWorkspaceRoot finds the root directory for a project
func DetectWorkspaceRoot(filePath string) string {
    dir := filepath.Dir(filePath)

    // Check for common workspace markers
    markers := []string{
        ".git",           // Git repository
        "go.mod",         // Go module
        "Cargo.toml",     // Rust project
        "package.json",   // Node project
        "pyproject.toml", // Python project
        ".vscode",        // VS Code workspace
    }

    for {
        // Check if any marker exists
        for _, marker := range markers {
            if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
                return dir
            }
        }

        // Move up one directory
        parent := filepath.Dir(dir)
        if parent == dir {
            // Reached filesystem root
            break
        }
        dir = parent
    }

    // Default to directory of file
    return filepath.Dir(filePath)
}

// DetectProjectLanguages scans a directory to find which languages are used
func DetectProjectLanguages(rootDir string) []string {
    var languages []string

    checks := map[string]func(string) bool{
        "go":         func(d string) bool { return fileExists(d, "go.mod") || hasFiles(d, "*.go") },
        "python":     func(d string) bool { return hasFiles(d, "*.py") },
        "javascript": func(d string) bool { return fileExists(d, "package.json") || hasFiles(d, "*.js") },
        "typescript": func(d string) bool { return hasFiles(d, "*.ts") || hasFiles(d, "*.tsx") },
        "rust":       func(d string) bool { return fileExists(d, "Cargo.toml") },
    }

    for lang, check := range checks {
        if check(rootDir) {
            languages = append(languages, lang)
        }
    }

    return languages
}

func fileExists(dir, name string) bool {
    _, err := os.Stat(filepath.Join(dir, name))
    return err == nil
}

func hasFiles(dir, pattern string) bool {
    matches, err := filepath.Glob(filepath.Join(dir, pattern))
    return err == nil && len(matches) > 0
}
```

**Usage:**
```go
// In Initialize method:
workspaceRoot := DetectWorkspaceRoot(filePath)
c.rootURI = "file://" + workspaceRoot

// Optionally pre-start all project language servers:
languages := DetectProjectLanguages(workspaceRoot)
for _, lang := range languages {
    go func(l string) {
        _, _ = manager.GetClient(ctx, l)
    }(lang)
}
```

---

## 🧪 Testing Strategy

### Manual Testing Checklist

**Day 3:**
```bash
# Python
cd ~/test_python
wilson "Create a Python FastAPI server with type hints"
# Verify: get_diagnostics called, type errors caught

# JavaScript
cd ~/test_js
wilson "Create a React counter component with hooks"
# Verify: JSX syntax validated, undefined vars caught

# TypeScript
cd ~/test_ts
wilson "Create a TypeScript Express API with interfaces"
# Verify: Type checking works, imports validated

# Rust
cd ~/test_rust
wilson "Create a Rust CLI JSON parser"
# Verify: Clippy warnings shown, ownership errors caught
```

### Automated Testing

**Day 4:**
```bash
# Run full multi-language test suite
go test -v ./tests -run TestMultiLanguage

# Expected: All 4 languages pass
# Go: ✅ (baseline)
# Python: ✅ (Pyright diagnostics work)
# JavaScript: ✅ (TSServer diagnostics work)
# Rust: ✅ (rust-analyzer diagnostics work)
```

### Performance Benchmarks

**Day 6:**
```bash
# Measure language server startup times
go test -bench=BenchmarkLSPStartup

# Expected results:
# Go (gopls):         ~500ms
# Python (Pyright):   ~800ms
# JavaScript (TSS):   ~1s
# Rust (r-a):         ~5-30s (first time), ~1s (cached)
```

---

## 📊 Success Criteria

### Phase 4A: Core Multi-Language (Days 1-2)
- ✅ Enhanced language detection (extensions + shebangs)
- ✅ Fallback language server chains
- ✅ Language-specific initialization options
- ✅ All 4 new languages detected correctly

### Phase 4B: Testing Infrastructure (Day 3)
- ✅ Installation script works on macOS/Linux
- ✅ Health check command shows all servers
- ✅ Test suite passes for all 4 languages
- ✅ Sample files for each language

### Phase 4C: Agent Integration (Day 4)
- ✅ Multi-language prompts added
- ✅ LSP tool descriptions updated
- ✅ CodeAgent can work with all languages
- ✅ Auto-detection seamless

### Phase 4D: Documentation (Day 5)
- ✅ Multi-language guide complete
- ✅ Per-language configuration documented
- ✅ Troubleshooting section
- ✅ Usage examples for each language

### Phase 4E: Performance (Day 6)
- ✅ Lazy initialization verified
- ✅ Idle client cleanup working
- ✅ Resource monitoring added
- ✅ Performance benchmarks pass

### Phase 4F: Edge Cases (Day 7)
- ✅ Crash recovery working
- ✅ Mixed-language projects supported
- ✅ Workspace detection accurate
- ✅ Error messages helpful

---

## 🎯 Expected Benefits

### Before Phase 4:
- ❌ Go only
- ❌ Limited to Go ecosystem
- ❌ Can't help with polyglot projects
- ❌ Python/JS/Rust users excluded

### After Phase 4:
- ✅ Universal coding assistant (4 languages)
- ✅ Polyglot project support
- ✅ +200% addressable market (Python/JS devs)
- ✅ Real-time diagnostics for all languages
- ✅ Consistent experience across languages

**Estimated Impact:**
- **+200% user base** (Python/JS are more popular than Go)
- **+40% effectiveness per language** (LSP vs text-based)
- **100% feature parity** across all 4 languages

---

## 🚀 Future Enhancements (Phase 5+)

### Additional Languages
- C/C++ (clangd)
- Java (jdtls)
- C# (OmniSharp)
- Ruby (Solargraph)
- PHP (Intelephense)

### Advanced Features
- Cross-language symbol resolution (e.g., Go calling Python via gRPC)
- Polyglot refactoring (rename across Go + TypeScript API boundaries)
- Language-aware dependency updates
- Multi-language code review

---

## 📝 Implementation Timeline

| Day | Focus | Deliverables |
|-----|-------|--------------|
| 1-2 | Core multi-language support | Enhanced detection, fallback chains, init options |
| 3 | Testing infrastructure | Install script, health check, test suite |
| 4 | Agent integration | Prompts, tool updates, seamless auto-detection |
| 5 | Documentation | Guide, examples, troubleshooting |
| 6 | Performance | Lazy init, idle cleanup, monitoring |
| 7 | Edge cases | Crash recovery, mixed projects, error handling |

**Total:** 7 days for production-ready multi-language LSP support

---

## ⚠️ Risk Mitigation

### Risk 1: Language Server Not Installed
**Mitigation:** Graceful fallback, clear error messages, installation script

### Risk 2: Language Server Crashes
**Mitigation:** Auto-restart on next request, crash detection, helpful logs

### Risk 3: Slow Performance (Rust)
**Mitigation:** Clear user messaging, async initialization, caching

### Risk 4: Configuration Complexity
**Mitigation:** Sensible defaults, optional configuration, auto-detection

---

**Status:** Ready to implement
**Blockers:** Phase 1 (Go LSP) must be complete and stable
**Next Step:** Begin Phase 4A - Core Multi-Language Support

**Owner:** Wilson Development Team
**Last Updated:** October 26, 2025
