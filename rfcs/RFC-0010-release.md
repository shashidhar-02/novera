# RFC-0010: Release Engineering & Supply-Chain Security

**RFC Number:** 0010  
**Title:** Reproducible Builds, SLSA v1.2 Provenance, and Sigstore/Cosign Signing  
**Author:** ASTRA Architecture Team  
**Status:** Approved / v0.1.0  
**Target Subsystem:** `.github/workflows/` & Release Engineering  

---

## 1. Scope & Standards Baseline

This specification defines ASTRA's release pipeline aligned with:
- **SLSA v1.2 (Supply-chain Levels for Software Artifacts):** Build Level 2 compliance.
- **Sigstore / Cosign:** Cryptographic container image signing and transparency logging.
- **NIST SP 800-218 (SSDF):** Software Supply Chain Development Framework.

---

## 2. Release Pipeline Workflow

```text
Git Release Tag (v*.*.*)
        |
        v
[ Automated Isolated GitHub Runner ]
        |
        +---> Go Clean Build (-trimpath, CGO_ENABLED=0)
        +---> Generate SHA-256 Checksums
        +---> Generate CycloneDX / SPDX SBOM
        +---> Build Minimal OCI Container Image
        +---> Sign OCI Digest with Cosign Keyless
        +---> Publish SLSA Level 2 Build Provenance Attestation
```

---

## 3. Cryptographic Verification Command

Downstream operators can verify release authenticity using the Cosign CLI:

```bash
# Verify container image signature
cosign verify \
  --certificate-identity "https://github.com/astra-orchestration/astra/.github/workflows/release.yml@refs/tags/v0.1.0" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  ghcr.io/astra-orchestration/astra-scheduler:v0.1.0

# Verify SBOM attestation
cosign verify-attestation \
  --type spdxjson \
  ghcr.io/astra-orchestration/astra-scheduler:v0.1.0
```
