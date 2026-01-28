# Capability: WebAuthn Compliance

## Overview
This specification aligns the Virtual FIDO system with the [W3C Web Authentication (WebAuthn) Level 3](https://w3c.github.io/webauthn/) Recommendation.

## Requirements

### Requirement: User Presence (UP)
The authenticator MUST act as a user-verifying authenticator by default or upon configuration.

#### Scenario: User Presence Check
- **WHEN** an authenticator receives a `MakeCredential` or `GetAssertion` request
- **THEN** it SHALL require a "user presence" test (e.g., explicit approval signal) before returning success, unless the `up` option is strictly false (which is rare in WebAuthn).
- **AND** the `flags` byte in the authenticator data SHALL have the `UP` bit (0x01) set.

### Requirement: User Verification (UV)
The authenticator MAY support User Verification (e.g., PIN, Biometrics).

#### Scenario: UV Not Configured
- **WHEN** the authenticator is not configured with a PIN/Biometric
- **AND** the Relying Party requests `uv=required`
- **THEN** the authenticator SHALL return `CTAP2_ERR_CONSTRAINT_VIOLATION` (or appropriate error) or fail the UV check bit.

### Requirement: Attestation
The authenticator MUST support generating attestation objects.

#### Scenario: Self Attestation
- **WHEN** `attestation` is requested as `direct` or `none`
- **THEN** the authenticator SHALL return a valid attestation statement (can be self-signed/AAGUID-based for virtual devices).

### Requirement: Relying Party Processing
The authenticator MUST verify the scope of the Relying Party ID.

#### Scenario: RP ID Hash Verification
- **WHEN** receiving a request
- **THEN** the authenticator SHALL associate the credential with the RP ID hash provided.

### Requirement: Signature Counter
The authenticator MUST implement a signature counter to prevent cloning attacks.

#### Scenario: Counter Increment
- **WHEN** a signature is generated (assertion)
- **THEN** the global or credential-specific sign counter SHALL be incremented.
- **AND** the new counter value SHALL be included in the authenticator data.

### Requirement: Extension Processing
The authenticator SHOULD support standard extensions (e.g., `hmac-secret`, `credProtect`).

#### Scenario: Unknown Extensions
- **WHEN** an unknown extension is requested
- **THEN** the authenticator SHALL ignore it and proceed (extensions are optional by default unless critical).
