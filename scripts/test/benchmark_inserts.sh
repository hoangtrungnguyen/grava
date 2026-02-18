#!/bin/bash
set -e

echo "📊 Grava Insert Performance Benchmark"
echo "======================================"
echo ""

# Ensure test environment is ready
echo "🔄 Setting up test database..."
./scripts/setup_test_env.sh > /dev/null 2>&1
echo "✅ Test database ready"
echo ""

# Export DB URL for benchmarks
export DB_URL="root@tcp(127.0.0.1:3306)/test_grava?parseTime=true"

# Run benchmarks with detailed output
echo "🏃 Running benchmarks..."
echo ""

# Create a temporary file for benchmark results
RESULTS_FILE=$(mktemp)

# Run benchmarks and capture output
go test -bench=. -benchmem -benchtime=3s ./pkg/cmd -run=^$ > "$RESULTS_FILE" 2>&1

# Display results
cat "$RESULTS_FILE"

echo ""
echo "======================================"
echo "📈 Benchmark Summary"
echo "======================================"

# Extract and display key metrics
echo ""
echo "Average times per operation:"
grep "Benchmark" "$RESULTS_FILE" | awk '{
    printf "  %-35s %10s ns/op  (%s ops in %s)\n", $1, $3, $2, $4
}'

echo ""
echo "Memory allocation per operation:"
grep "Benchmark" "$RESULTS_FILE" | awk '{
    if (NF >= 6) {
        printf "  %-35s %10s B/op   %8s allocs/op\n", $1, $5, $7
    }
}'

echo ""
echo "======================================"
echo "📊 Specific Metrics (1000 item inserts)"
echo "======================================"

# Calculate metrics for bulk insert benchmark
BULK_LINE=$(grep "BenchmarkBulkInsert1000" "$RESULTS_FILE" || echo "")
if [ -n "$BULK_LINE" ]; then
    # Extract operations and ns/op
    OPS=$(echo "$BULK_LINE" | awk '{print $2}')
    NS_PER_OP=$(echo "$BULK_LINE" | awk '{print $3}')

    if [ -n "$OPS" ] && [ -n "$NS_PER_OP" ]; then
        # Calculate average time per single insert (ns/op is for 1000 items)
        AVG_PER_INSERT=$(echo "scale=2; $NS_PER_OP / 1000" | bc)
        MS_PER_1000=$(echo "scale=2; $NS_PER_OP / 1000000" | bc)

        echo "  Total operations:        $OPS iterations of 1000 inserts"
        echo "  Time per 1000 inserts:   ${MS_PER_1000} ms"
        echo "  Average per insert:      ${AVG_PER_INSERT} ns (~$(echo "scale=3; $AVG_PER_INSERT / 1000000" | bc) ms)"
        echo ""

        # Calculate throughput
        if [ $(echo "$NS_PER_OP > 0" | bc) -eq 1 ]; then
            INSERTS_PER_SEC=$(echo "scale=0; 1000000000 / $AVG_PER_INSERT" | bc)
            echo "  Throughput:              ~${INSERTS_PER_SEC} inserts/second"
        fi
    fi
else
    echo "  ⚠️  BenchmarkBulkInsert1000 results not found"
fi

echo ""
echo "======================================"
echo ""
echo "✅ Benchmark complete!"
echo ""
echo "💡 To run benchmarks manually:"
echo "   go test -bench=. -benchmem -benchtime=5s ./pkg/cmd"
echo ""
echo "💡 To benchmark a specific function:"
echo "   go test -bench=BenchmarkBulkInsert1000 -benchmem ./pkg/cmd"
echo ""

# Cleanup
rm -f "$RESULTS_FILE"
