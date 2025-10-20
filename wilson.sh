#!/bin/bash
# Wilson Launcher Script
# Automatically starts Ollama if needed and runs Wilson

# Dynamically resolve Wilson directory relative to script location
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WILSON_DIR="$SCRIPT_DIR/go"
WILSON_BIN="$WILSON_DIR/wilson"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Ollama is running
check_ollama() {
    if pgrep -x "ollama" > /dev/null; then
        return 0  # Running
    else
        return 1  # Not running
    fi
}

# Start Ollama in background
start_ollama() {
    echo -e "${YELLOW}Starting Ollama...${NC}"
    nohup ollama serve > /dev/null 2>&1 &

    # Wait for Ollama to be ready
    for i in {1..10}; do
        if curl -s http://localhost:11434/api/version > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Ollama is ready${NC}"
            return 0
        fi
        sleep 1
    done

    echo -e "${RED}✗ Failed to start Ollama${NC}"
    return 1
}

# Check if Wilson binary exists
if [ ! -f "$WILSON_BIN" ]; then
    echo -e "${RED}✗ Wilson binary not found at: $WILSON_BIN${NC}"
    echo -e "${YELLOW}Building Wilson...${NC}"
    cd "$WILSON_DIR" || exit 1
    go build -o wilson main.go
    if [ $? -ne 0 ]; then
        echo -e "${RED}✗ Build failed${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Wilson built successfully${NC}"
fi

# Check and start Ollama if needed
if check_ollama; then
    echo -e "${GREEN}✓ Ollama is already running${NC}"
else
    if ! start_ollama; then
        echo -e "${RED}Please start Ollama manually: ollama serve${NC}"
        exit 1
    fi
fi

# Run Wilson from the correct directory (for config loading)
echo -e "${GREEN}Starting Wilson...${NC}"
echo ""
cd "$WILSON_DIR" || exit 1
exec "$WILSON_BIN" "$@"
