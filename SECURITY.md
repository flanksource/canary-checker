# Security Policy

## Reporting a Vulnerability

If you discover any security vulnerabilities within this project, please report them to our team immediately. We appreciate your help in making this project more secure for everyone.

To report a vulnerability, please follow these steps:

1. **Email**: Send an email to our security team at [security@flanksource.com](mailto:security@flanksource.com) with a detailed description of the vulnerability.
2. **Subject Line**: Use the subject line "Security Vulnerability Report" to ensure prompt attention.
3. **Information**: Provide as much information as possible about the vulnerability, including steps to reproduce it and any supporting documentation or code snippets.
4. **Confidentiality**: We prioritize the confidentiality of vulnerability reports. Please avoid publicly disclosing the issue until we have had an opportunity to address it.

Our team will respond to your report as soon as possible and work towards a solution. We appreciate your responsible disclosure and cooperation in maintaining the security of this project.

Thank you for your contribution to the security of this project!

**Note:** This project follows responsible disclosure practices.

## Vulnerability Scanning

This project uses `govulncheck` to scan for known vulnerabilities in Go dependencies.

### Running Vulnerability Scans

To scan for vulnerabilities, run:

```bash
govulncheck ./...
```

For verbose output showing all details:

```bash
govulncheck -show verbose ./...
```

## Known Non-Exploitable Vulnerabilities

The following vulnerabilities are present in indirect dependencies but are **NOT exploitable** in this codebase:

### GO-2022-0635: AWS S3 Crypto SDK In-band Key Negotiation Issue

- **CVE**: CVE-2020-8912
- **Package**: `github.com/aws/aws-sdk-go/service/s3/s3crypto`
- **Status**: Not exploitable
- **Reason**:
  - We don't import or use the `s3crypto` package
  - The vulnerable functions (`NewDecryptionClient`, `NewEncryptionClient`) are never called
  - Only present as an indirect dependency from other packages
  - Verified with `govulncheck`: "your code doesn't appear to call these vulnerabilities"

### GO-2022-0646: AWS S3 Crypto SDK CBC Padding Oracle Issue

- **CVE**: CVE-2020-8911
- **Package**: `github.com/aws/aws-sdk-go/service/s3/s3crypto`
- **Status**: Not exploitable
- **Reason**:
  - We don't import or use the `s3crypto` package
  - We don't use the vulnerable EncryptionClient with AES-CBC cipher
  - Only present as an indirect dependency from other packages
  - Verified with `govulncheck`: "your code doesn't appear to call these vulnerabilities"

### Indirect Dependencies

These vulnerabilities come from the following indirect dependencies:
- `github.com/flanksource/artifacts`
- `github.com/flanksource/duty`
- `github.com/flanksource/kommons`
- `github.com/opensearch-project/opensearch-go/v2`
- `github.com/prometheus/alertmanager`
- `gocloud.dev`

### Configuration Files

- `.govulncheck.yaml` - Documents vulnerability exceptions for reference
- `.osv-scanner.toml` - Configuration for OSV-Scanner with ignored vulnerabilities

### Verification

The exploitability of these vulnerabilities has been verified using:

1. **Code analysis**: Manual inspection confirms we don't use the `s3crypto` package
2. **Dependency graph**: `go mod graph | grep aws-sdk-go` shows only indirect dependencies
3. **govulncheck**: Automated scan confirms vulnerable code is not called
4. **go mod why**: Confirms the package is not directly required

To verify yourself:

```bash
# Check if we use the vulnerable package
grep -r "s3crypto" . --include="*.go"

# Check why the package is included
go mod why github.com/aws/aws-sdk-go

# Run vulnerability check
govulncheck -show verbose ./...
```

**Last Verified**: 2025-10-29 - 0 exploitable vulnerabilities
