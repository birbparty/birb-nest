#!/bin/bash

# Birb Nest Performance Benchmarking Script
# This script runs Go benchmarks and generates reports

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
BENCH_TIME="${BENCH_TIME:-10s}"
BENCH_COUNT="${BENCH_COUNT:-3}"
BENCH_PATTERN="${BENCH_PATTERN:-.}"
OUTPUT_DIR="${OUTPUT_DIR:-tests/load/results}"
CPUPROFILE="${CPUPROFILE:-false}"
MEMPROFILE="${MEMPROFILE:-false}"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run benchmarks
run_benchmarks() {
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_file="${OUTPUT_DIR}/bench_${timestamp}.txt"
    local json_file="${OUTPUT_DIR}/bench_${timestamp}.json"
    
    print_info "🐦 Running Birb Nest benchmarks..."
    print_info "Pattern: $BENCH_PATTERN"
    print_info "Time: $BENCH_TIME per benchmark"
    print_info "Count: $BENCH_COUNT runs"
    
    # Build benchmark flags
    local bench_flags="-bench=$BENCH_PATTERN -benchtime=$BENCH_TIME -count=$BENCH_COUNT -benchmem"
    
    # Add profiling flags if requested
    if [ "$CPUPROFILE" = "true" ]; then
        bench_flags="$bench_flags -cpuprofile=${OUTPUT_DIR}/cpu_${timestamp}.prof"
        print_info "CPU profiling enabled"
    fi
    
    if [ "$MEMPROFILE" = "true" ]; then
        bench_flags="$bench_flags -memprofile=${OUTPUT_DIR}/mem_${timestamp}.prof"
        print_info "Memory profiling enabled"
    fi
    
    # Run benchmarks and save output
    print_info "Starting benchmark run..."
    
    # Run with both text and JSON output
    go test -v ./tests/load/... $bench_flags | tee "$output_file"
    
    # Also generate JSON output for easier parsing
    go test -json ./tests/load/... $bench_flags > "$json_file" 2>&1 || true
    
    print_success "Benchmarks completed!"
    print_info "Results saved to:"
    print_info "  - Text: $output_file"
    print_info "  - JSON: $json_file"
    
    # Generate summary
    generate_summary "$output_file" "$timestamp"
}

# Function to generate benchmark summary
generate_summary() {
    local bench_file=$1
    local timestamp=$2
    local summary_file="${OUTPUT_DIR}/summary_${timestamp}.txt"
    
    print_info "Generating benchmark summary..."
    
    {
        echo "==================================="
        echo "BIRB NEST BENCHMARK SUMMARY"
        echo "==================================="
        echo "Timestamp: $(date)"
        echo ""
        echo "REDIS CACHE PERFORMANCE:"
        echo "-----------------------"
        grep -E "^Benchmark.*Redis" "$bench_file" | column -t
        echo ""
        echo "POSTGRESQL PERFORMANCE:"
        echo "----------------------"
        grep -E "^Benchmark.*Postgres" "$bench_file" | column -t
        echo ""
        echo "NATS QUEUE PERFORMANCE:"
        echo "----------------------"
        grep -E "^Benchmark.*NATS" "$bench_file" | column -t
        echo ""
        echo "CONCURRENT ACCESS:"
        echo "-----------------"
        grep -E "^Benchmark.*Concurrent" "$bench_file" | column -t
        echo ""
        echo "END-TO-END PERFORMANCE:"
        echo "----------------------"
        grep -E "^Benchmark.*EndToEnd" "$bench_file" | column -t
        echo ""
        echo "MEMORY ALLOCATION:"
        echo "-----------------"
        grep -E "^Benchmark.*Memory" "$bench_file" | column -t
    } > "$summary_file"
    
    # Display summary
    cat "$summary_file"
    
    print_success "Summary saved to: $summary_file"
}

# Function to compare benchmarks
compare_benchmarks() {
    local file1=$1
    local file2=$2
    
    if [ ! -f "$file1" ] || [ ! -f "$file2" ]; then
        print_error "Both benchmark files must exist for comparison"
        return 1
    fi
    
    print_info "Comparing benchmarks..."
    print_info "Old: $file1"
    print_info "New: $file2"
    
    # Use benchstat if available
    if command -v benchstat &> /dev/null; then
        benchstat "$file1" "$file2"
    else
        print_warning "benchstat not installed. Install with: go install golang.org/x/perf/cmd/benchstat@latest"
        print_info "Showing simple diff instead:"
        diff -u "$file1" "$file2" || true
    fi
}

# Function to analyze profiles
analyze_profiles() {
    local prof_file=$1
    local prof_type=$2
    
    if [ ! -f "$prof_file" ]; then
        print_error "Profile file not found: $prof_file"
        return 1
    fi
    
    print_info "Analyzing $prof_type profile: $prof_file"
    
    case "$prof_type" in
        "cpu")
            go tool pprof -top "$prof_file" | head -20
            print_info "For interactive analysis, run: go tool pprof $prof_file"
            ;;
        "mem")
            go tool pprof -alloc_space -top "$prof_file" | head -20
            print_info "For interactive analysis, run: go tool pprof -alloc_space $prof_file"
            ;;
        *)
            print_error "Unknown profile type: $prof_type"
            return 1
            ;;
    esac
}

# Function to run quick benchmark
quick_bench() {
    print_info "Running quick benchmark (1s per test)..."
    BENCH_TIME="1s" BENCH_COUNT="1" run_benchmarks
}

# Function to run specific benchmark categories
run_category() {
    local category=$1
    
    case "$category" in
        "redis")
            BENCH_PATTERN="BenchmarkRedis" run_benchmarks
            ;;
        "postgres")
            BENCH_PATTERN="BenchmarkPostgres" run_benchmarks
            ;;
        "nats")
            BENCH_PATTERN="BenchmarkNATS" run_benchmarks
            ;;
        "concurrent")
            BENCH_PATTERN="BenchmarkConcurrent" run_benchmarks
            ;;
        "e2e")
            BENCH_PATTERN="BenchmarkEndToEnd" run_benchmarks
            ;;
        *)
            print_error "Unknown category: $category"
            print_info "Available categories: redis, postgres, nats, concurrent, e2e"
            return 1
            ;;
    esac
}

# Main execution
main() {
    case "${1:-run}" in
        "run")
            run_benchmarks
            ;;
        "quick")
            quick_bench
            ;;
        "category")
            run_category "${2:-all}"
            ;;
        "compare")
            compare_benchmarks "$2" "$3"
            ;;
        "analyze-cpu")
            analyze_profiles "$2" "cpu"
            ;;
        "analyze-mem")
            analyze_profiles "$2" "mem"
            ;;
        "clean")
            print_info "Cleaning benchmark results..."
            rm -rf "$OUTPUT_DIR"/*
            print_success "Cleaned!"
            ;;
        *)
            print_error "Usage: $0 [run|quick|category|compare|analyze-cpu|analyze-mem|clean]"
            echo ""
            echo "Commands:"
            echo "  run              - Run full benchmark suite"
            echo "  quick            - Run quick benchmark (1s per test)"
            echo "  category <name>  - Run specific category (redis|postgres|nats|concurrent|e2e)"
            echo "  compare <f1> <f2> - Compare two benchmark results"
            echo "  analyze-cpu <file> - Analyze CPU profile"
            echo "  analyze-mem <file> - Analyze memory profile"
            echo "  clean            - Clean all benchmark results"
            echo ""
            echo "Environment variables:"
            echo "  BENCH_TIME    - Benchmark duration (default: 10s)"
            echo "  BENCH_COUNT   - Number of runs (default: 3)"
            echo "  BENCH_PATTERN - Benchmark pattern (default: .)"
            echo "  OUTPUT_DIR    - Output directory (default: tests/load/results)"
            echo "  CPUPROFILE    - Enable CPU profiling (default: false)"
            echo "  MEMPROFILE    - Enable memory profiling (default: false)"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
