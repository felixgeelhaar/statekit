# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

We take the security of statekit seriously. If you discover a security vulnerability, please report it responsibly.

### How to Report

1. **Do NOT open a public GitHub issue** for security vulnerabilities
2. **Use GitHub's private vulnerability reporting**:
   - Go to the [Security tab](https://github.com/klarlabs-studio/statekit/security)
   - Click "Report a vulnerability"
   - Provide a detailed description of the vulnerability

### What to Include

- A clear description of the vulnerability
- Steps to reproduce the issue
- Affected versions
- Any potential mitigations you've identified
- Your contact information (optional, for follow-up questions)

### What to Expect

- **Acknowledgment**: We will acknowledge receipt of your report within 48 hours
- **Assessment**: We will investigate and assess the severity within 7 days
- **Resolution**: We aim to release a fix within 30 days for critical vulnerabilities
- **Disclosure**: We will coordinate with you on public disclosure timing

### Scope

The following are in scope for security reports:

- The statekit library (`go.klarlabs.de/statekit`)
- All sub-packages (http, otel, viz, health, metrics, etc.)
- The CLI tool (`cmd/statekit`)
- Example code in the `examples/` directory

### Out of Scope

- Vulnerabilities in dependencies (report these to the respective maintainers)
- Issues in documentation or non-code assets
- Social engineering attacks

## Security Best Practices

When using statekit in your applications:

1. **Validate event payloads** before processing
2. **Use context timeouts** for distributed interpreters
3. **Implement proper access controls** when using HTTP handlers
4. **Store sensitive context data** securely (encryption at rest)
5. **Review action and guard implementations** for injection vulnerabilities

## Dependencies

We use Dependabot to monitor and update dependencies. Security updates are prioritized and typically merged within 24-48 hours of notification.
