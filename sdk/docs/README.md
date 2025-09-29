# Birb-Nest SDK Documentation

Welcome to the Birb-Nest SDK documentation! This guide helps you navigate all available documentation based on your role and needs.

## 🚨 For On-Call Engineers

**Start Here When Things Break:**
1. [ON_CALL_RUNBOOK.md](./ON_CALL_RUNBOOK.md) - Emergency procedures with step-by-step fixes
2. [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) - Error dictionary explaining every error message

## 🔧 For Operations Teams

**Deployment and Daily Operations:**
1. [DEPLOYMENT.md](./DEPLOYMENT.md) - Step-by-step deployment guide (Docker, Kubernetes, Manual)
2. [OPERATIONS.md](./OPERATIONS.md) - Configuration, health monitoring, common operations
3. [MONITORING.md](./MONITORING.md) - Setting up metrics, alerts, and dashboards
4. [LOGGING.md](./LOGGING.md) - Log management, analysis, and retention

## 👩‍💻 For Developers

**SDK Integration and Usage:**
1. [SDK README](../README.md) - Quick start, configuration, API reference
2. [Basic Examples](../examples/basic/) - Simple cache operations, error handling
3. [Advanced Examples](../examples/advanced/) - High availability, load balancing
4. [WASM Examples](../examples/wasm/) - Browser-based usage

## 📋 Quick Reference Matrix

| I need to... | Read this |
|-------------|-----------|
| Fix a production issue NOW | [ON_CALL_RUNBOOK.md](./ON_CALL_RUNBOOK.md) |
| Understand an error message | [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) |
| Deploy for the first time | [DEPLOYMENT.md](./DEPLOYMENT.md) |
| Configure the SDK | [OPERATIONS.md](./OPERATIONS.md#configuration) |
| Set up monitoring | [MONITORING.md](./MONITORING.md) |
| Analyze logs | [LOGGING.md](./LOGGING.md) |
| Learn SDK basics | [SDK README](../README.md#5-minute-quick-start) |
| See code examples | [Examples Directory](../examples/) |

## 📁 Documentation Structure

```
sdk/
├── README.md                    # Developer quick start guide
├── docs/                        # Operational documentation
│   ├── README.md               # This file
│   ├── ON_CALL_RUNBOOK.md     # Emergency procedures
│   ├── TROUBLESHOOTING.md     # Error dictionary
│   ├── OPERATIONS.md          # Operations guide
│   ├── DEPLOYMENT.md          # Deployment instructions
│   ├── MONITORING.md          # Monitoring setup
│   └── LOGGING.md             # Log management
├── examples/                    # Code examples
│   ├── basic/                  # Basic usage examples
│   ├── advanced/              # Advanced patterns
│   └── wasm/                  # WebAssembly examples
└── templates/                   # Ready-to-use configurations
    ├── docker-compose.example.yml
    ├── grafana-dashboard.json
    └── prometheus-alerts.yml
```

## 🎯 Role-Based Learning Paths

### For On-Call Engineers
1. Read [ON_CALL_RUNBOOK.md](./ON_CALL_RUNBOOK.md) completely
2. Bookmark [TROUBLESHOOTING.md](./TROUBLESHOOTING.md) for reference
3. Familiarize yourself with the monitoring dashboard
4. Practice the emergency procedures during quiet times

### For DevOps/SRE
1. Start with [DEPLOYMENT.md](./DEPLOYMENT.md)
2. Review [OPERATIONS.md](./OPERATIONS.md) for configuration
3. Set up monitoring using [MONITORING.md](./MONITORING.md)
4. Configure logging per [LOGGING.md](./LOGGING.md)

### For Developers
1. Complete the [5-minute quick start](../README.md#5-minute-quick-start)
2. Run the [basic examples](../examples/basic/)
3. Review [error handling patterns](./TROUBLESHOOTING.md)
4. Implement monitoring in your app

## 🚀 Getting Started Checklist

- [ ] Choose deployment method (Docker Compose recommended)
- [ ] Copy template configurations from `templates/`
- [ ] Update passwords and environment variables
- [ ] Deploy using [DEPLOYMENT.md](./DEPLOYMENT.md)
- [ ] Import Grafana dashboard
- [ ] Set up Prometheus alerts
- [ ] Test health endpoints
- [ ] Document any customizations

## 📞 Support Escalation Path

1. **Level 1**: Check documentation (especially troubleshooting guide)
2. **Level 2**: Search logs for error patterns
3. **Level 3**: Check monitoring dashboards for anomalies
4. **Level 4**: Follow escalation in [ON_CALL_RUNBOOK.md](./ON_CALL_RUNBOOK.md)

## 🔄 Keeping Documentation Updated

When you encounter new issues or solutions:
1. Document the problem and fix
2. Update the relevant guide
3. Add to troubleshooting if it's a new error
4. Share with the team

## Key Features of This Documentation

✅ **Zero-Code Operations** - Deploy and operate without reading code  
✅ **Plain English** - Technical concepts explained simply  
✅ **Step-by-Step** - Clear procedures for every task  
✅ **Error Dictionary** - Every error explained with solutions  
✅ **Emergency Ready** - Quick reference for crisis situations  
✅ **Templates Included** - Production-ready configurations  

Remember: This documentation is designed so someone who has never seen the code can successfully deploy, operate, and troubleshoot the system. If something is unclear, that's a bug in the documentation - please report it!
