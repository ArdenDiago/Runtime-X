#!/bin/bash

# Runtime-X Master Run Script
# This script starts both the backend and frontend services.

# Colors for logging
BLUE='\033[0;34m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting Runtime-X Project...${NC}"

# Function to kill background processes on exit
cleanup() {
    echo -e "\n${BLUE}Stopping all services...${NC}"
    kill $BACKEND_PID $FRONTEND_PID
    exit
}

# Trap Ctrl+C (SIGINT) and SIGTERM
trap cleanup SIGINT SIGTERM

# Start Backend
echo -e "${GREEN}Starting Backend on :8080...${NC}"
go run cmd/main.go &
BACKEND_PID=$!

# Start Frontend
echo -e "${GREEN}Starting Frontend on :3000...${NC}"
(cd frontend && go run cmd/main.go) &
FRONTEND_PID=$!

echo -e "${BLUE}Both services are running.${NC}"
echo -e "Backend: [http://localhost:8080](http://localhost:8080)"
echo -e "Frontend: [http://localhost:3000](http://localhost:3000)"
echo -e "${BLUE}Press Ctrl+C to stop both services.${NC}"

# Wait for background processes
wait
