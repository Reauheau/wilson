#!/bin/bash
# Install Language Servers for Wilson LSP Support
# Supports: Go, Python, JavaScript/TypeScript, Rust

set -e

echo "🔧 Installing Language Servers for Wilson"
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check prerequisites
echo "Checking prerequisites..."
command -v npm >/dev/null 2>&1 || { echo -e "${RED}❌ npm not found. Install Node.js first.${NC}"; exit 1; }
command -v pip3 >/dev/null 2>&1 || echo -e "${YELLOW}⚠️  pip3 not found. Python support will be limited.${NC}"
command -v cargo >/dev/null 2>&1 || echo -e "${YELLOW}⚠️  cargo not found. Rust support will be limited.${NC}"
echo ""

# Go (gopls)
echo "📦 Installing gopls (Go language server)..."
if command -v go >/dev/null 2>&1; then
    go install golang.org/x/tools/gopls@latest
    echo -e "${GREEN}✅ gopls installed${NC}"
else
    echo -e "${RED}❌ Go not found, skipping gopls${NC}"
fi
echo ""

# Python (Pyright)
echo "📦 Installing pyright (Python language server)..."
npm install -g pyright
echo -e "${GREEN}✅ pyright installed${NC}"
echo ""

echo "📦 Installing pylsp (Python LSP fallback)..."
if command -v pip3 >/dev/null 2>&1; then
    pip3 install 'python-lsp-server[all]'
    echo -e "${GREEN}✅ pylsp installed${NC}"
else
    echo -e "${YELLOW}⚠️  Skipping pylsp (pip3 not found)${NC}"
fi
echo ""

# JavaScript/TypeScript
echo "📦 Installing typescript-language-server..."
npm install -g typescript-language-server typescript
echo -e "${GREEN}✅ typescript-language-server installed${NC}"
echo ""

# Rust (rust-analyzer)
echo "📦 Installing rust-analyzer..."
if command -v rustup >/dev/null 2>&1; then
    rustup component add rust-analyzer
    echo -e "${GREEN}✅ rust-analyzer installed via rustup${NC}"
elif command -v cargo >/dev/null 2>&1; then
    cargo install rust-analyzer
    echo -e "${GREEN}✅ rust-analyzer installed via cargo${NC}"
else
    echo -e "${YELLOW}⚠️  Rust not found, skipping rust-analyzer${NC}"
fi
echo ""

echo -e "${GREEN}✅ Language server installation complete!${NC}"
echo ""
echo "Installed servers:"
command -v gopls >/dev/null 2>&1 && echo -e "  ${GREEN}✅ gopls${NC} (Go)" || echo -e "  ${RED}❌ gopls${NC} (Go)"
command -v pyright-langserver >/dev/null 2>&1 && echo -e "  ${GREEN}✅ pyright${NC} (Python)" || echo -e "  ${RED}❌ pyright${NC} (Python)"
command -v pylsp >/dev/null 2>&1 && echo -e "  ${GREEN}✅ pylsp${NC} (Python fallback)" || echo -e "  ${YELLOW}⚠️  pylsp${NC} (Python fallback)"
command -v typescript-language-server >/dev/null 2>&1 && echo -e "  ${GREEN}✅ typescript-language-server${NC} (JS/TS)" || echo -e "  ${RED}❌ typescript-language-server${NC} (JS/TS)"
command -v rust-analyzer >/dev/null 2>&1 && echo -e "  ${GREEN}✅ rust-analyzer${NC} (Rust)" || echo -e "  ${RED}❌ rust-analyzer${NC} (Rust)"
echo ""
echo "Run './wilson' and try LSP tools on Python, JavaScript, TypeScript, or Rust files!"
