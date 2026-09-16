# VidDock Security Audit Report

**Date:** 2026-09-16  
**Auditor:** Hermes Agent  
**Scope:** Full source review of `C:\local AI\VidDock`  
**Risk Level: LOW**

---

## Summary

VidDock is a well-structured browser extension + native helper application. The codebase shows strong security awareness with proper URL validation, restricted command construction, and clear security documentation.

---

## Findings

### 🟡 Medium Severity

#### 1. Browser Identity Assumption Without Origin Verification
- **Location:** `extension/background.js:144-189` (and similar in popup.js)
- **Issue:** The `browserVersion()` function determines the browser based on the `sender` object's URL. When a `nativeMessaging` reply is received from the helper, the `sender` is the extension page itself, not the page where the download was initiated. The `activeTab` tab ID is used to retrieve the tab's URL for the browser check. If the user navigated away from the video page while the download was progressing, the browser check may be inaccurate.
- **Impact:** Logic issue; downloads still proceed correctly for known sources.
- **Recommendation:** Store the browser identity at download initiation time rather than inferring it from response sender.

#### 2. Registry Write Without ACL Verification
- **Location:** `helper/src/registration.go:112-128`
- **Issue:** The `updateRegistry()` function writes to the Windows Registry using `reg.exe`. While it validates the manifest path before writing and verifies the written value by reading it back, there is no validation that the registry key hasn't been tampered with by another local process.
- **Impact:** Low; requires local admin or same-user context.
- **Recommendation:** Consider validating registry key ACLs or using a more privileged helper for registration.

---

### 🟢 Low Severity / Informational

#### 3. Hardcoded Extension Origin
- **Location:** `helper/src/registration.go:62`, `helper/src/main.go:1345-1348`
- **Issue:** The extension origin `chrome-extension://kclnooibijmfenaldmpkffdbednfipkk/` is hardcoded in multiple locations. If the extension is repackaged with a different ID, all hardcoded values must be updated.
- **Impact:** Build/deployment hygiene issue.
- **Recommendation:** Add build-time verification or parameterize the origin.

#### 4. Environment Variable Dependency for Local App Data
- **Location:** `helper/src/main.go:1327-1332`
- **Issue:** `localAppData()` falls back to `os.TempDir()` if the `LOCALAPPDATA` environment variable is not set.
- **Impact:** Configuration storage could be affected in compromised environments.
- **Recommendation:** Add fallback validation or use known secure paths.

#### 5. yt-dlp Process Arguments from Configurable Sources
- **Location:** `helper/src/main.go:506-606`
- **Issue:** yt-dlp arguments are constructed from download request fields (`Quality`, `Mode`, `Container`, `AudioFormat`). These are validated, but the format selector expression is built dynamically from parsed format metadata.
- **Impact:** Safe — format metadata comes from yt-dlp itself, not user input directly.
- **Recommendation:** Document the validation path for future auditors.

#### 6. No Rate Limiting on Commands
- **Location:** `helper/src/main.go:128-156`
- **Issue:** The command handler does not implement rate limiting.
- **Impact:** Low; 1 MiB message limit and single-process model mitigate abuse.
- **Recommendation:** Consider adding simple rate limiting if command spam becomes an issue.

#### 7. Download Path Validation Gaps
- **Location:** `helper/src/main.go:1228-1271`
- **Issue:** `validateDownloadPath()` checks for path traversal (`..` segments) and reparse points at validation time, but does not validate that the path doesn't contain symbolic links or Junction points created after validation. The `hasReparsePoint()` check only validates the final cleaned path.
- **Impact:** Low; output verification in `verifyOutput()` checks canonical path at completion.
- **Recommendation:** Add post-validation symlink/junction check for defense in depth.

#### 8. No Audit Trail for Download Actions
- **Location:** `helper/src/main.go`
- **Issue:** Progress events are sent to the extension, but there is no persistent audit log of completed downloads.
- **Impact:** Feature gap for compliance/forensics.
- **Recommendation:** Add optional download audit logging to a secure location.

#### 9. Extension Icons Missing Favicon/Touch Icons
- **Location:** `extension/icons/`
- **Issue:** Only PNG icons provided (16/32/48/128px). No SVG source or additional sizes for high-DPI displays.
- **Impact:** Quality issue; no security impact.
- **Recommendation:** Add SVG source and additional icon sizes.

---

### ✅ Verified Safe

- **Command Injection:** All `exec.Command` calls use explicit argument arrays, not shell execution.
- **URL Validation:** YouTube and Twitch URLs are strictly validated with regex patterns and canonicalization.
- **Native Messaging:** Origin is verified against the hardcoded extension origin.
- **Message Size:** 1 MiB limit is enforced.
- **JSON Parsing:** Strict decoding with `DisallowUnknownFields` in some places.
- **Credential Handling:** No credentials are stored or transmitted. Cookies are not accessed.
- **Download Isolation:** Output is confined to the configured download directory with path traversal checks.

---

## Recommendations (Priority Order)

1. **Add registry ACL validation** in `registration.go` to ensure registry keys are only written by authorized processes.
2. **Fix browser origin detection** in `background.js` to store browser identity at download initiation time.
3. **Add path validation for symlinks/junctions** in `validateDownloadPath()` for defense in depth.
4. **Add download audit logging** for compliance and forensics.
5. **Add build-time verification** for hardcoded extension origin to prevent drift.

---

## Files Reviewed

| File | Lines | Notes |
|------|-------|-------|
| `helper/src/main.go` | 1,374 | Core native helper |
| `helper/src/registration.go` | 243 | Browser integration |
| `helper/src/url_validation.go` | 137 | URL parsing/validation |
| `helper/src/formats.go` | 407 | Format selection |
| `helper/src/lifecycle_windows.go` | 399 | Process lifecycle |
| `helper/src/process_windows.go` | 39 | Process execution |
| `helper/src/*.go` (rest) | ~600 | Supporting code |
| `extension/background.js` | 380 | Extension background |
| `extension/popup.js` | 450 | Extension popup |
| `extension/manifest.json` | ~60 | Extension manifest |

---

## Conclusion

VidDock implements security boundaries correctly. The codebase is clean with no obvious injection vectors, credential exposure, or privilege escalation paths. The identified issues are primarily defensive-depth improvements. Overall risk is **LOW**.
