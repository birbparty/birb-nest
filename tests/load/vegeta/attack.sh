#!/bin/bash

# Birb Nest Vegeta Load Testing Script
# This script runs various load test scenarios using Vegeta

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default values
RATE="${RATE:-50}"
DURATION="${DURATION:-30s}"
OUTPUT_DIR="${OUTPUT_DIR:-tests/load/results}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
SCENARIO="${SCENARIO:-mixed}"

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

# Function to check if vegeta is installed
check_vegeta() {
    if ! command -v vegeta &> /dev/null; then
        print_error "Vegeta is not installed. Please install it first:"
        echo "  brew install vegeta    # macOS"
        echo "  go install github.com/tsenart/vegeta/v12@latest    # Using Go"
        exit 1
    fi
    print_info "Vegeta version: $(vegeta version)"
}

# Function to run a vegeta attack
run_attack() {
    local scenario=$1
    local rate=$2
    local duration=$3
    local timestamp=$(date +%Y%m%d_%H%M%S)
    local output_prefix="${OUTPUT_DIR}/vegeta_${scenario}_${timestamp}"
    
    print_info "Running $scenario scenario..."
    print_info "Rate: $rate req/s, Duration: $duration"
    
    case "$scenario" in
        "quick")
            # Quick smoke test
            vegeta attack \
                -targets=tests/load/vegeta/targets.txt \
                -rate=$rate \
                -duration=$duration \
                -output="${output_prefix}.bin" \
                -name="Quick Smoke Test"
            ;;
            
        "read-heavy")
            # Generate read-heavy targets
            echo "# Read-heavy load test" > /tmp/vegeta_reads.txt
            for i in {1..100}; do
                echo "GET ${BASE_URL}/v1/cache/test-key-$i" >> /tmp/vegeta_reads.txt
            done
            
            vegeta attack \
                -targets=/tmp/vegeta_reads.txt \
                -rate=$rate \
                -duration=$duration \
                -output="${output_prefix}.bin" \
                -name="Read Heavy Test"
            ;;
            
        "write-heavy")
            # Generate write-heavy targets
            echo "# Write-heavy load test" > /tmp/vegeta_writes.txt
            for i in {1..50}; do
                cat >> /tmp/vegeta_writes.txt <<EOF
POST ${BASE_URL}/v1/cache/test-key-write-$i
Content-Type: application/json
@tests/load/vegeta/payloads/medium.json

EOF
            done
            
            vegeta attack \
                -targets=/tmp/vegeta_writes.txt \
                -rate=$rate \
                -duration=$duration \
                -output="${output_prefix}.bin" \
                -name="Write Heavy Test"
            ;;
            
        "mixed")
            # Use the default targets file (mixed workload)
            vegeta attack \
                -targets=tests/load/vegeta/targets.txt \
                -rate=$rate \
                -duration=$duration \
                -output="${output_prefix}.bin" \
                -name="Mixed Workload Test"
            ;;
            
        "stress")
            # High-rate stress test
            print_warning "Running stress test with high request rate!"
            vegeta attack \
                -targets=tests/load/vegeta/targets.txt \
                -rate=1000 \
                -duration=1m \
                -output="${output_prefix}.bin" \
                -name="Stress Test"
            ;;
            
        *)
            print_error "Unknown scenario: $scenario"
            return 1
            ;;
    esac
    
    # Generate reports
    print_info "Generating reports..."
    
    # Text report
    vegeta report "${output_prefix}.bin" > "${output_prefix}_report.txt"
    
    # JSON report for programmatic access
    vegeta report -type=json "${output_prefix}.bin" > "${output_prefix}_report.json"
    
    # Generate histogram
    vegeta report -type=hist[0,10ms,25ms,50ms,100ms,200ms,500ms,1s,2s,5s] "${output_prefix}.bin" > "${output_prefix}_histogram.txt"
    
    # Plot if requested
    if [ "$PLOT" = "true" ]; then
        vegeta plot "${output_prefix}.bin" > "${output_prefix}_plot.html"
        print_info "HTML plot saved to: ${output_prefix}_plot.html"
    fi
    
    # Display summary
    echo ""
    print_success "Attack completed! Summary:"
    echo "----------------------------------------"
    cat "${output_prefix}_report.txt"
    echo "----------------------------------------"
    
    print_info "Full results saved to: ${output_prefix}_*"
}

# Function to run comparative tests
run_comparison() {
    print_info "Running comparative load tests..."
    
    local scenarios=("read-heavy" "write-heavy" "mixed")
    local rates=(50 100 200 500)
    
    for scenario in "${scenarios[@]}"; do
        for rate in "${rates[@]}"; do
            print_info "Testing $scenario at $rate req/s..."
            run_attack "$scenario" "$rate" "30s"
            sleep 10  # Cool down between tests
        done
    done
    
    print_success "Comparative tests completed!"
}

# Function to warm up the cache
warmup_cache() {
    print_info "Warming up cache..."
    
    # Create warmup targets
    echo "# Cache warmup" > /tmp/vegeta_warmup.txt
    for i in {1..1000}; do
        cat >> /tmp/vegeta_warmup.txt <<EOF
POST ${BASE_URL}/v1/cache/warmup-key-$i
Content-Type: application/json
@tests/load/vegeta/payloads/small.json

EOF
    done
    
    # Run warmup at moderate rate
    vegeta attack \
        -targets=/tmp/vegeta_warmup.txt \
        -rate=100 \
        -duration=10s \
        -output="${OUTPUT_DIR}/warmup.bin"
    
    print_success "Cache warmup completed!"
}

# Main execution
main() {
    print_info "🐦 Birb Nest Vegeta Load Testing 🐦"
    
    # Check prerequisites
    check_vegeta
    
    # Parse command line arguments
    case "${1:-$SCENARIO}" in
        "quick")
            run_attack "quick" "$RATE" "$DURATION"
            ;;
        "read-heavy")
            run_attack "read-heavy" "$RATE" "$DURATION"
            ;;
        "write-heavy")
            run_attack "write-heavy" "$RATE" "$DURATION"
            ;;
        "mixed")
            run_attack "mixed" "$RATE" "$DURATION"
            ;;
        "stress")
            run_attack "stress" "$RATE" "$DURATION"
            ;;
        "comparison")
            run_comparison
            ;;
        "warmup")
            warmup_cache
            ;;
        "all")
            warmup_cache
            sleep 5
            run_attack "read-heavy" "$RATE" "$DURATION"
            sleep 5
            run_attack "write-heavy" "$RATE" "$DURATION"
            sleep 5
            run_attack "mixed" "$RATE" "$DURATION"
            ;;
        *)
            print_error "Usage: $0 [quick|read-heavy|write-heavy|mixed|stress|comparison|warmup|all]"
            echo ""
            echo "Environment variables:"
            echo "  RATE=50         Request rate (req/s)"
            echo "  DURATION=30s    Test duration"
            echo "  OUTPUT_DIR=...  Output directory for results"
            echo "  BASE_URL=...    Base URL for API"
            echo "  PLOT=true       Generate HTML plots"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
