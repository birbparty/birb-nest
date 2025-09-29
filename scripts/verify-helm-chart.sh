#!/bin/bash

# Helm Chart Verification Script
# This script validates the birb-nest Helm chart structure and YAML files

set -e

CHART_DIR="charts/birb-nest"
ERRORS=0

echo "🔍 Verifying Helm Chart: $CHART_DIR"
echo "============================================"

# Function to report errors
report_error() {
    echo "❌ ERROR: $1"
    ERRORS=$((ERRORS + 1))
}

# Function to report success
report_success() {
    echo "✅ $1"
}

# Check if chart directory exists
if [ ! -d "$CHART_DIR" ]; then
    report_error "Chart directory $CHART_DIR not found"
    exit 1
fi

# Check required files
echo -e "\n📋 Checking required files..."
REQUIRED_FILES=(
    "Chart.yaml"
    "values.yaml"
    "templates/_helpers.tpl"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ -f "$CHART_DIR/$file" ]; then
        report_success "Found $file"
    else
        report_error "Missing required file: $file"
    fi
done

# Validate Chart.yaml
echo -e "\n📊 Validating Chart.yaml..."
if [ -f "$CHART_DIR/Chart.yaml" ]; then
    # Check for required fields
    for field in "apiVersion" "name" "version" "type"; do
        if grep -q "^$field:" "$CHART_DIR/Chart.yaml"; then
            report_success "Chart.yaml contains $field"
        else
            report_error "Chart.yaml missing required field: $field"
        fi
    done
    
    # Check apiVersion
    if grep -q "^apiVersion: v2" "$CHART_DIR/Chart.yaml"; then
        report_success "Chart uses Helm v3 (apiVersion: v2)"
    else
        report_error "Chart should use apiVersion: v2 for Helm v3"
    fi
fi

# Check template files
echo -e "\n📄 Checking template files..."
TEMPLATE_FILES=(
    "deployment.yaml"
    "service.yaml"
    "serviceaccount.yaml"
    "networkpolicy.yaml"
    "backup-cronjob.yaml"
)

for file in "${TEMPLATE_FILES[@]}"; do
    if [ -f "$CHART_DIR/templates/$file" ]; then
        report_success "Found template: $file"
        
        # Basic YAML syntax check
        if python3 -c "import yaml; yaml.safe_load(open('$CHART_DIR/templates/$file'))" 2>/dev/null; then
            # Skip - templates have Helm syntax
            :
        fi
    else
        report_error "Missing template: $file"
    fi
done

# Validate values.yaml
echo -e "\n⚙️  Validating values.yaml..."
if [ -f "$CHART_DIR/values.yaml" ]; then
    # Check YAML syntax
    if python3 -c "import yaml; yaml.safe_load(open('$CHART_DIR/values.yaml'))" 2>/dev/null; then
        report_success "values.yaml has valid YAML syntax"
    else
        report_error "values.yaml has invalid YAML syntax"
    fi
    
    # Check for important values
    for value in "mode" "replicaCount" "image" "service" "networkPolicy"; do
        if grep -q "^$value:" "$CHART_DIR/values.yaml"; then
            report_success "values.yaml contains $value configuration"
        else
            report_error "values.yaml missing $value configuration"
        fi
    done
fi

# Check example values file
echo -e "\n📝 Checking replica example values..."
if [ -f "$CHART_DIR/values-replica-example.yaml" ]; then
    report_success "Found values-replica-example.yaml"
    
    # Verify it's set to replica mode
    if grep -q "^mode: replica" "$CHART_DIR/values-replica-example.yaml"; then
        report_success "Example is configured for replica mode"
    else
        report_error "Example should have mode: replica"
    fi
    
    # Verify PostgreSQL is disabled
    if grep -A2 "^postgresql:" "$CHART_DIR/values-replica-example.yaml" | grep -q "enabled: false"; then
        report_success "PostgreSQL is disabled in replica example"
    else
        report_error "PostgreSQL should be disabled in replica example"
    fi
else
    report_error "Missing values-replica-example.yaml"
fi

# Check README
echo -e "\n📚 Checking documentation..."
if [ -f "$CHART_DIR/README.md" ]; then
    report_success "Found README.md"
    
    # Check for important sections
    for section in "Prerequisites" "Installation" "Configuration" "Monitoring"; do
        if grep -q "## $section" "$CHART_DIR/README.md"; then
            report_success "README contains $section section"
        else
            report_error "README missing $section section"
        fi
    done
else
    report_error "Missing README.md"
fi

# Security checks
echo -e "\n🔒 Security checks..."
# Check network policy is enabled by default
if grep -A2 "^networkPolicy:" "$CHART_DIR/values.yaml" | grep -q "enabled: true"; then
    report_success "Network policy is enabled by default"
else
    report_error "Network policy should be enabled by default"
fi

# Check service type is ClusterIP
if grep -A2 "^service:" "$CHART_DIR/values.yaml" | grep -q "type: ClusterIP"; then
    report_success "Service type is ClusterIP (internal only)"
else
    report_error "Service type should be ClusterIP for internal access"
fi

# Template structure validation
echo -e "\n🏗️  Validating template structure..."
for template in "$CHART_DIR/templates"/*.yaml; do
    if [ -f "$template" ]; then
        filename=$(basename "$template")
        
        # Check for proper Helm template syntax
        if grep -q "{{" "$template"; then
            report_success "$filename uses Helm templating"
        else
            # ServiceAccount might be simple
            if [ "$filename" != "serviceaccount.yaml" ]; then
                report_error "$filename doesn't use Helm templating"
            fi
        fi
        
        # Check for proper resource definitions
        if grep -q "^apiVersion:" "$template" || grep -q "{{.*if.*}}" "$template"; then
            report_success "$filename has proper structure"
        else
            report_error "$filename missing proper Kubernetes resource structure"
        fi
    fi
done

# Summary
echo -e "\n============================================"
if [ $ERRORS -eq 0 ]; then
    echo "✅ Chart validation passed! No errors found."
    exit 0
else
    echo "❌ Chart validation failed with $ERRORS error(s)."
    exit 1
fi
