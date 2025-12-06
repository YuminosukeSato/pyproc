# pyproc Security Guide

## Overview

This document outlines security considerations, threat model, and best practices for deploying pyproc in production environments.

## Threat Model

### Assets to Protect

1. **Python Worker Processes** - Execution environment for user code
2. **Unix Domain Sockets** - Communication channel
3. **Go Application** - Process manager and API gateway
4. **User Data** - Data processed by Python functions

### Potential Threats

| Threat | Impact | Likelihood | Mitigation |
|--------|--------|------------|------------|
| Arbitrary code execution in Python | Critical | Medium | Input validation, sandboxing |
| Socket hijacking | High | Low | File permissions, access control |
| Resource exhaustion | Medium | Medium | Resource limits, monitoring |
| Information disclosure | High | Low | Proper error handling, logging |
| Denial of Service | Medium | Medium | Rate limiting, backpressure |

## Security Architecture

### Process Isolation

```
┌─────────────────────────────────────────┐
│         Go Process (Supervisor)          │
│                                          │
│  - Manages worker lifecycle              │
│  - Enforces resource limits              │
│  - Implements access control             │
└─────────────────────────────────────────┘
                    │
           Process Boundary
                    │
┌─────────────────────────────────────────┐
│      Python Workers (Isolated)           │
│                                          │
│  - Separate process space                │
│  - Limited privileges                    │
│  - No direct system access               │
└─────────────────────────────────────────┘
```

### Communication Security

Unix Domain Sockets provide:
- **No network exposure** - Local only communication
- **Filesystem permissions** - OS-level access control
- **Process authentication** - UID/GID verification

## Best Practices

### 1. Run with Least Privilege

```bash
# Create dedicated user
useradd -r -s /bin/false pyproc

# Set ownership
chown pyproc:pyproc /var/run/pyproc

# Run workers as unprivileged user
su -s /bin/bash pyproc -c "python worker.py"
```

### 2. Socket Permission Management

```go
// Restrictive socket permissions
cfg := pyproc.SocketConfig{
    Dir:         "/var/run/pyproc",
    Permissions: 0600, // Owner read/write only
}
```

### 3. Input Validation

Always validate input in Python workers:

```python
@expose
def process_data(req):
    # Validate input types
    if not isinstance(req.get("data"), list):
        raise ValueError("Invalid input: data must be a list")
    
    # Validate input size
    if len(req["data"]) > MAX_INPUT_SIZE:
        raise ValueError("Input too large")
    
    # Sanitize input values
    data = [sanitize(item) for item in req["data"]]
    
    return process_safe(data)
```

### 4. Resource Limits

#### Memory Limits

```yaml
# Kubernetes
resources:
  limits:
    memory: "1Gi"
    
# Docker
docker run --memory="1g" --memory-swap="1g" myapp
```

#### CPU Limits

```yaml
# Kubernetes
resources:
  limits:
    cpu: "1000m"
    
# Docker
docker run --cpus="1.0" myapp
```

#### File Descriptor Limits

```python
import resource

# Set limits in Python worker
resource.setrlimit(resource.RLIMIT_NOFILE, (1024, 1024))
resource.setrlimit(resource.RLIMIT_NPROC, (100, 100))
```

### 5. Dependency Management

```python
# requirements.txt - Pin versions
numpy==1.21.0
pandas==1.3.0
scikit-learn==0.24.2

# Regular updates
pip install --upgrade pip
pip install -r requirements.txt
```

### 6. Error Handling

Never expose internal details in errors:

```python
@expose
def secure_function(req):
    try:
        # Process request
        return process(req)
    except InternalError as e:
        # Log full error internally
        logger.error(f"Internal error: {e}")
        # Return generic error to client
        raise ValueError("Processing failed")
```

## Sandboxing Options

### 1. Container Isolation

```dockerfile
FROM python:3.11-slim
RUN useradd -r pyproc
USER pyproc
# Drop all capabilities
RUN setcap -r /usr/local/bin/python3.11
```

### 2. seccomp Profiles

```json
{
  "defaultAction": "SCMP_ACT_ERRNO",
  "architectures": ["SCMP_ARCH_X86_64"],
  "syscalls": [
    {
      "names": ["read", "write", "open", "close"],
      "action": "SCMP_ACT_ALLOW"
    }
  ]
}
```

### 3. AppArmor/SELinux

```
# AppArmor profile
profile pyproc_worker {
  # Allow reading worker script
  /app/worker.py r,
  
  # Allow socket access
  /var/run/pyproc/* rw,
  
  # Deny network access
  deny network,
  
  # Deny raw socket
  deny capability net_raw,
}
```

## Monitoring and Auditing

### Security Metrics

Monitor these security-relevant metrics:

- Failed authentication attempts
- Unusual resource consumption
- Error rate spikes
- Process crashes
- Socket connection failures

### Audit Logging

```go
// Log security events
logger.Info("worker_started", 
    "worker_id", workerID,
    "user", os.Getuid(),
    "socket", socketPath,
)

logger.Warn("invalid_request",
    "method", req.Method,
    "error", err,
    "source", conn.RemoteAddr(),
)
```

## Incident Response

### Detection

1. Monitor logs for anomalies
2. Set up alerts for security events
3. Track resource usage patterns
4. Review audit logs regularly

### Response Plan

1. **Isolate** - Stop affected workers
2. **Investigate** - Analyze logs and memory dumps
3. **Remediate** - Patch vulnerabilities
4. **Recovery** - Restart with fixes
5. **Review** - Post-mortem analysis

## Security Checklist

### Development

- [ ] Input validation in all exposed functions
- [ ] Error messages don't leak sensitive info
- [ ] Dependencies are pinned and verified
- [ ] Code reviewed for security issues
- [ ] Static analysis tools run (bandit, gosec)

### Deployment

- [ ] Running as non-root user
- [ ] Socket permissions set to 0600
- [ ] Resource limits configured
- [ ] Sandboxing enabled (containers/seccomp)
- [ ] Monitoring and alerting configured

### Operations

- [ ] Regular dependency updates
- [ ] Security patches applied promptly
- [ ] Audit logs reviewed
- [ ] Incident response plan tested
- [ ] Backup and recovery procedures

## Common Vulnerabilities

### 1. Code Injection

**Risk**: Executing arbitrary Python code

**Mitigation**:
```python
# NEVER do this
exec(req["code"])  # DANGEROUS!

# Instead, use predefined functions
ALLOWED_FUNCTIONS = {"predict", "process"}
if req["method"] in ALLOWED_FUNCTIONS:
    result = ALLOWED_FUNCTIONS[req["method"]](req["body"])
```

### 2. Path Traversal

**Risk**: Accessing files outside intended directory

**Mitigation**:
```python
import os

def safe_path(base, user_path):
    # Resolve to absolute path
    path = os.path.join(base, user_path)
    real_path = os.path.realpath(path)
    
    # Ensure it's within base directory
    if not real_path.startswith(os.path.realpath(base)):
        raise ValueError("Invalid path")
    
    return real_path
```

### 3. Resource Exhaustion

**Risk**: DoS through resource consumption

**Mitigation**:
```python
import signal
import resource

# Set timeout
signal.alarm(30)  # 30 second timeout

# Limit memory
resource.setrlimit(resource.RLIMIT_AS, (1024*1024*1024, -1))  # 1GB
```

## Compliance Considerations

### Data Protection

- Implement encryption at rest for sensitive data
- Use TLS for any network communication
- Follow data retention policies
- Implement audit trails

### Access Control

- Implement RBAC for multi-tenant scenarios
- Log all access attempts
- Regular access reviews
- Principle of least privilege

## Security Updates

Stay informed about security updates:

- Watch the [pyproc repository](https://github.com/YuminosukeSato/pyproc)
- Subscribe to security advisories
- Regular dependency scanning
- Vulnerability assessments

## Reporting Security Issues

If you discover a security vulnerability:

1. Do NOT open a public issue
2. Email security details to: security@example.com
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Suggested fix (if any)

We aim to respond within 48 hours and provide a fix within 7 days for critical issues.