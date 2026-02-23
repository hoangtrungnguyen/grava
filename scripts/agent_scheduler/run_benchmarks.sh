#!/bin/bash

# Benchmark runner script for PearceKellyScheduler
# Runs performance tests and generates report

set -e  # Exit on error

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$SCRIPT_DIR"

echo "======================================"
echo "Agent Scheduler - Benchmark Runner"
echo "======================================"
echo ""

# Check Python version
echo "Checking Python version..."
PYTHON_CMD=""
if command -v python3 &> /dev/null; then
    PYTHON_CMD="python3"
    PYTHON_VERSION=$(python3 --version)
elif command -v python &> /dev/null; then
    PYTHON_CMD="python"
    PYTHON_VERSION=$(python --version)
else
    echo "ERROR: Python not found. Please install Python 3.7+"
    exit 1
fi

echo "  Using: $PYTHON_CMD ($PYTHON_VERSION)"
echo ""

# Run unit tests first
echo "======================================"
echo "Step 1: Running Unit Tests"
echo "======================================"
echo ""

cd "$SCRIPT_DIR/../.."
$PYTHON_CMD -m agent_scheduler.test_scheduler
TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "ERROR: Unit tests failed. Fix tests before running benchmarks."
    exit 1
fi

echo ""
echo "✅ All unit tests passed!"
echo ""

# Run benchmarks
echo "======================================"
echo "Step 2: Running Performance Benchmarks"
echo "======================================"
echo ""

START_TIME=$(date +%s)

$PYTHON_CMD -m agent_scheduler.benchmark

BENCHMARK_EXIT_CODE=$?
END_TIME=$(date +%s)
ELAPSED=$((END_TIME - START_TIME))

if [ $BENCHMARK_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "ERROR: Benchmarks failed with exit code $BENCHMARK_EXIT_CODE"
    exit 1
fi

echo ""
echo "✅ Benchmarks completed in ${ELAPSED} seconds"
echo ""

# Check if results file exists
RESULTS_FILE="scripts/agent_scheduler/benchmark_results.json"
if [ ! -f "$RESULTS_FILE" ]; then
    echo "ERROR: Benchmark results file not found: $RESULTS_FILE"
    exit 1
fi

echo "======================================"
echo "Step 3: Generating Report"
echo "======================================"
echo ""

cd "$SCRIPT_DIR"
$PYTHON_CMD generate_report.py

REPORT_EXIT_CODE=$?

if [ $REPORT_EXIT_CODE -ne 0 ]; then
    echo ""
    echo "ERROR: Report generation failed"
    exit 1
fi

echo ""
echo "✅ Report generated successfully"
echo ""

# Final summary
echo "======================================"
echo "Benchmark Results"
echo "======================================"
echo ""
echo "Results file:  $RESULTS_FILE"
echo "Report file:   docs/epics/artifacts/AgentScheduler_Benchmark_Report.md"
echo ""
echo "Total time:    ${ELAPSED} seconds"
echo ""
echo "✅ All benchmarks completed successfully!"
echo ""

exit 0
