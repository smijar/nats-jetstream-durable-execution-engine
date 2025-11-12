#!/bin/bash

set -e

echo "========================================="
echo "End-to-End Test: Durable Execution"
echo "========================================="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Start HelloService in background
echo -e "${YELLOW}Starting HelloService...${NC}"
./bin/services > /tmp/service.log 2>&1 &
SERVICE_PID=$!
sleep 2

if ! kill -0 $SERVICE_PID 2>/dev/null; then
    echo -e "${RED}✗ HelloService failed to start${NC}"
    cat /tmp/service.log
    exit 1
fi
echo -e "${GREEN}✓ HelloService started (PID: $SERVICE_PID)${NC}"

# Start Processor in background
echo -e "${YELLOW}Starting Processor...${NC}"
./bin/processor > /tmp/processor.log 2>&1 &
PROCESSOR_PID=$!
sleep 3

if ! kill -0 $PROCESSOR_PID 2>/dev/null; then
    echo -e "${RED}✗ Processor failed to start${NC}"
    cat /tmp/processor.log
    kill $SERVICE_PID 2>/dev/null
    exit 1
fi
echo -e "${GREEN}✓ Processor started (PID: $PROCESSOR_PID)${NC}"

# Submit a command
echo ""
echo -e "${YELLOW}Submitting workflow command...${NC}"
./bin/client > /tmp/client.log 2>&1
INVOCATION_ID=$(grep "Invocation ID:" /tmp/client.log | awk '{print $NF}')
echo -e "${GREEN}✓ Command submitted: $INVOCATION_ID${NC}"

# Wait for execution
echo -e "${YELLOW}Waiting for execution...${NC}"
sleep 3

# Check processor logs for success
echo ""
echo "========================================="
echo "Processor Logs:"
echo "========================================="
tail -15 /tmp/processor.log

echo ""
echo "========================================="
echo "Service Logs:"
echo "========================================="
tail -5 /tmp/service.log

# Verify success
if grep -q "Execution completed successfully" /tmp/processor.log; then
    echo ""
    echo -e "${GREEN}=========================================${NC}"
    echo -e "${GREEN}✓ TEST PASSED: Workflow executed successfully!${NC}"
    echo -e "${GREEN}=========================================${NC}"

    # Show the greeting message
    if grep -q "Hello workflow completed" /tmp/processor.log; then
        GREETING=$(grep "Hello workflow completed" /tmp/processor.log | tail -1)
        echo ""
        echo -e "${GREEN}Message: ${GREETING}${NC}"
    fi
else
    echo ""
    echo -e "${RED}=========================================${NC}"
    echo -e "${RED}✗ TEST FAILED: Workflow did not complete${NC}"
    echo -e "${RED}=========================================${NC}"
fi

# Cleanup
echo ""
echo -e "${YELLOW}Cleaning up...${NC}"
kill $PROCESSOR_PID $SERVICE_PID 2>/dev/null || true
wait $PROCESSOR_PID $SERVICE_PID 2>/dev/null || true
echo -e "${GREEN}✓ Cleanup complete${NC}"

echo ""
echo "Logs saved to /tmp/processor.log and /tmp/service.log"
