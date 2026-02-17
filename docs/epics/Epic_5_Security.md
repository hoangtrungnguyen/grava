# Epic 5: Security Implementation and Access Control

**Goal:** Implement zero-trust security and granular permissions before exposing to autonomous agents.

**Success Criteria:**
- mTLS authentication enforced on server
- RBAC configured with human and agent roles
- Agents cannot execute destructive operations
- Complete audit trail of all mutations
- Failed authentication logged and alerted

## User Stories

### 5.1 Mutual TLS Configuration
**As a** security engineer  
**I want to** enforce mutual TLS on the Dolt server  
**So that** only authorized clients can connect

**Acceptance Criteria:**
- Server requires client certificates (listener.require_client_cert: true)
- CA certificate authority set up for signing client certs
- Client certificates distributed to authorized agents
- Unauthorized connection attempts rejected with clear error
- Certificate rotation procedure documented
- TLS 1.3 minimum version enforced

### 5.2 Role-Based Access Control (RBAC)
**As a** system administrator  
**I want to** define granular database permissions  
**So that** agents cannot accidentally destroy data

**Acceptance Criteria:**
- `human_admin` role with full schema control (DDL/DML)
- `ai_agent` role with restricted permissions (SELECT, INSERT, UPDATE only)
- `ai_agent` role explicitly denied DELETE, DROP, TRUNCATE
- `read_only` role for monitoring and reporting tools
- Roles enforced at database connection level
- Role assignment tracked in privileges.db

### 5.3 Immutable Audit Logging
**As a** compliance officer  
**I want to** log every database mutation with actor identity  
**So that** I can trace who made what changes and when

**Acceptance Criteria:**
- Events table captures all INSERT/UPDATE/DELETE operations
- Actor field populated with authenticated identity (human or agent ID)
- Old and new values stored as JSON for complete diff
- Audit log is append-only (DELETE disabled)
- Timestamp precision to millisecond level
- Query interface for audit trail search

### 5.4 Privilege Enforcement Testing
**As a** QA engineer  
**I want to** verify that permission boundaries are respected  
**So that** security cannot be bypassed

**Acceptance Criteria:**
- Test suite verifies agent role cannot DELETE
- Test suite verifies agent role cannot DROP tables
- Test suite verifies agent role cannot ALTER schema
- Test suite verifies unauthorized certificates are rejected
- Penetration testing completed with clean results
- Security vulnerability scan passed

### 5.5 Incident Response and Rollback
**As a** system administrator  
**I want to** quickly rollback malicious or erroneous changes  
**So that** data integrity can be restored immediately

**Acceptance Criteria:**
- `DOLT_REVERT()` procedure tested and documented
- Rollback can target specific commit hashes
- Rollback preserves audit trail of the revert itself
- Alert system notifies admins of suspicious activity patterns
- Recovery time objective (RTO) <5 minutes documented
