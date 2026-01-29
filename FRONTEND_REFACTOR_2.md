# NmapTracker Feature Additions: Scope Management, XML Import, UI Fixes

## Objective

Add three pieces of functionality:
1. **Scope Management**: Define IP/CIDR scope rules per project, evaluate hosts against them
2. **XML Import via UI**: Upload nmap XML files through the frontend instead of CLI-only
3. **UI Fixes**: Fix expandable content (notes, script output) so they don't break layout

---

## Feature 1: Scope Management

### Overview

Currently `in_scope` is a static boolean on hosts with no actual logic. We need:
- Per-project scope definitions (list of IPs and/or CIDR blocks)
- Hosts evaluated against scope on import
- Ability to re-evaluate existing hosts when scope rules change

### Database

There's already a `scope_definition` table in the schema. Verify it exists or create:

```sql
CREATE TABLE IF NOT EXISTS scope_definition (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    definition TEXT NOT NULL,  -- The IP or CIDR (e.g., "10.0.0.0/24" or "192.168.1.50")
    type TEXT NOT NULL,        -- "cidr" or "ip"
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_scope_project ON scope_definition(project_id);
```

### Backend API

#### List Scope Rules
```
GET /api/projects/{id}/scope
Response: [
  { "id": 1, "definition": "10.0.0.0/24", "type": "cidr", "created_at": "..." },
  { "id": 2, "definition": "192.168.1.50", "type": "ip", "created_at": "..." }
]
```

#### Add Scope Rules (bulk)
```
POST /api/projects/{id}/scope
Body: {
  "definitions": ["10.0.0.0/24", "192.168.1.50", "172.16.0.0/16"]
}
Response: {
  "added": 3,
  "rules": [...]
}
```

The backend should:
- Parse each definition to determine if it's an IP or CIDR
- Validate format (use `net.ParseIP` and `net.ParseCIDR` in Go)
- Skip duplicates (same definition already exists for project)
- Return error for invalid entries

#### Delete Scope Rule
```
DELETE /api/projects/{id}/scope/{scopeId}
Response: 204 No Content
```

#### Re-evaluate Scope
```
POST /api/projects/{id}/scope/evaluate
Response: {
  "updated": 12,
  "in_scope": 45,
  "out_of_scope": 8
}
```

This endpoint:
1. Fetches all scope rules for the project
2. Fetches all hosts for the project
3. For each host, checks if IP matches any scope rule
4. Updates `in_scope` flag accordingly
5. Returns counts

### Scope Matching Logic

In Go (likely in `internal/scope/` - there's already a `matcher.go`):

```go
package scope

import (
    "net"
    "net/netip"
)

type Rule struct {
    Definition string
    Type       string // "ip" or "cidr"
    prefix     netip.Prefix
    addr       netip.Addr
}

type Matcher struct {
    rules []Rule
}

func NewMatcher(definitions []string) (*Matcher, error) {
    var rules []Rule
    for _, def := range definitions {
        rule := Rule{Definition: def}
        
        // Try parsing as CIDR first
        if prefix, err := netip.ParsePrefix(def); err == nil {
            rule.Type = "cidr"
            rule.prefix = prefix
            rules = append(rules, rule)
            continue
        }
        
        // Try parsing as IP
        if addr, err := netip.ParseAddr(def); err == nil {
            rule.Type = "ip"
            rule.addr = addr
            rules = append(rules, rule)
            continue
        }
        
        // Invalid - skip or return error depending on preference
        // For robustness, skip invalid entries
    }
    
    return &Matcher{rules: rules}, nil
}

func (m *Matcher) InScope(ip string) bool {
    if len(m.rules) == 0 {
        // No rules defined = everything in scope (or out of scope - pick one)
        // Recommend: no rules = everything in scope
        return true
    }
    
    addr, err := netip.ParseAddr(ip)
    if err != nil {
        return false
    }
    
    for _, rule := range m.rules {
        switch rule.Type {
        case "cidr":
            if rule.prefix.Contains(addr) {
                return true
            }
        case "ip":
            if rule.addr == addr {
                return true
            }
        }
    }
    
    return false
}
```

### Update Import Logic

When importing nmap XML, after parsing hosts:
1. Load scope rules for the project
2. Create matcher
3. For each host being imported, set `in_scope = matcher.InScope(host.IP)`

The existing import logic in `internal/importer/` should be updated to accept the matcher and use it.

### Frontend: Scope Section on Dashboard

Add a collapsible section to `project.html`:

```html
<div class="card" style="margin-top: 24px;">
  <div class="card-header" style="cursor: pointer;" onclick="toggleScope()">
    <h3 class="card-title">Scope Definition</h3>
    <span id="scope-toggle-icon">▼</span>
  </div>
  
  <div id="scope-content" class="scope-content">
    <!-- Input area -->
    <div class="scope-input-section">
      <label for="scope-input">Add IPs or CIDR blocks (one per line)</label>
      <textarea id="scope-input" rows="4" placeholder="10.0.0.0/24&#10;192.168.1.50&#10;172.16.0.0/16"></textarea>
      <div class="scope-actions">
        <button class="btn btn-primary" onclick="addScopeRules()">Add to Scope</button>
      </div>
    </div>
    
    <!-- Current rules -->
    <div class="scope-rules-section">
      <div class="scope-rules-header">
        <span class="label">Current Scope Rules</span>
        <button class="btn btn-secondary" onclick="reEvaluateScope()">Re-evaluate Hosts</button>
      </div>
      <ul id="scope-rules-list" class="scope-rules-list">
        <!-- Populated by JS -->
      </ul>
      <p id="scope-empty" class="text-muted" style="display: none;">
        No scope rules defined. All hosts will be considered in-scope.
      </p>
    </div>
  </div>
</div>
```

CSS for scope section:
```css
.scope-content {
  padding-top: 20px;
}

.scope-content.collapsed {
  display: none;
}

.scope-input-section {
  margin-bottom: 24px;
}

.scope-input-section textarea {
  width: 100%;
  margin-bottom: 12px;
}

.scope-rules-section {
  border-top: 1px solid var(--border);
  padding-top: 20px;
}

.scope-rules-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.scope-rules-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.scope-rule-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: var(--bg-base);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-family: var(--font-mono);
  font-size: 14px;
}

.scope-rule-item .rule-type {
  font-size: 11px;
  color: var(--text-muted);
  text-transform: uppercase;
  margin-left: 12px;
}

.scope-rule-item .delete-btn {
  background: transparent;
  border: none;
  color: var(--red);
  cursor: pointer;
  padding: 4px 8px;
  font-size: 16px;
}

.scope-rule-item .delete-btn:hover {
  color: #f87171;
}
```

JavaScript for scope management:
```javascript
// In dashboard.js or app.js

async function loadScopeRules() {
  const projectId = getProjectId();
  const rules = await api(`/projects/${projectId}/scope`);
  renderScopeRules(rules);
}

function renderScopeRules(rules) {
  const list = document.getElementById('scope-rules-list');
  const empty = document.getElementById('scope-empty');
  
  if (rules.length === 0) {
    list.innerHTML = '';
    empty.style.display = 'block';
    return;
  }
  
  empty.style.display = 'none';
  list.innerHTML = rules.map(rule => `
    <li class="scope-rule-item">
      <span>
        ${escapeHtml(rule.definition)}
        <span class="rule-type">${rule.type}</span>
      </span>
      <button class="delete-btn" onclick="deleteScopeRule(${rule.id})" title="Remove">×</button>
    </li>
  `).join('');
}

async function addScopeRules() {
  const input = document.getElementById('scope-input');
  const lines = input.value.split('\n')
    .map(l => l.trim())
    .filter(l => l.length > 0);
  
  if (lines.length === 0) {
    showToast('Enter at least one IP or CIDR', 'error');
    return;
  }
  
  const projectId = getProjectId();
  
  try {
    const result = await api(`/projects/${projectId}/scope`, {
      method: 'POST',
      body: JSON.stringify({ definitions: lines })
    });
    
    showToast(`Added ${result.added} scope rule(s)`, 'success');
    input.value = '';
    loadScopeRules();
  } catch (err) {
    showToast('Failed to add scope rules: ' + err.message, 'error');
  }
}

async function deleteScopeRule(ruleId) {
  const projectId = getProjectId();
  
  try {
    await api(`/projects/${projectId}/scope/${ruleId}`, {
      method: 'DELETE'
    });
    showToast('Scope rule removed', 'success');
    loadScopeRules();
  } catch (err) {
    showToast('Failed to remove rule: ' + err.message, 'error');
  }
}

async function reEvaluateScope() {
  const projectId = getProjectId();
  
  try {
    const result = await api(`/projects/${projectId}/scope/evaluate`, {
      method: 'POST'
    });
    showToast(`Updated ${result.updated} hosts (${result.in_scope} in scope, ${result.out_of_scope} out of scope)`, 'success');
    // Optionally reload stats
    loadDashboardStats();
  } catch (err) {
    showToast('Failed to re-evaluate scope: ' + err.message, 'error');
  }
}

function toggleScope() {
  const content = document.getElementById('scope-content');
  const icon = document.getElementById('scope-toggle-icon');
  content.classList.toggle('collapsed');
  icon.textContent = content.classList.contains('collapsed') ? '▶' : '▼';
}

// Call on page load
loadScopeRules();
```

---

## Feature 2: XML Import via UI

### Backend API

```
POST /api/projects/{id}/import
Content-Type: multipart/form-data
Body: file (the .xml file)

Response: {
  "success": true,
  "filename": "scan.xml",
  "hosts_imported": 42,
  "ports_imported": 156,
  "hosts_in_scope": 38,
  "hosts_out_scope": 4
}
```

Handler implementation:
```go
func (s *Server) apiImportXML(w http.ResponseWriter, r *http.Request) {
    projectID, err := parseProjectID(r)
    if err != nil {
        jsonError(w, "invalid project id", http.StatusBadRequest)
        return
    }
    
    // Verify project exists
    project, found, err := s.DB.GetProjectByID(projectID)
    if err != nil || !found {
        jsonError(w, "project not found", http.StatusNotFound)
        return
    }
    
    // Parse multipart form (limit to 50MB)
    if err := r.ParseMultipartForm(50 << 20); err != nil {
        jsonError(w, "failed to parse form", http.StatusBadRequest)
        return
    }
    
    file, header, err := r.FormFile("file")
    if err != nil {
        jsonError(w, "no file provided", http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    // Read file content
    content, err := io.ReadAll(file)
    if err != nil {
        jsonError(w, "failed to read file", http.StatusInternalServerError)
        return
    }
    
    // Parse XML
    observations, err := importer.ParseXMLBytes(content)
    if err != nil {
        jsonError(w, "failed to parse XML: "+err.Error(), http.StatusBadRequest)
        return
    }
    
    // Load scope rules and create matcher
    scopeRules, err := s.DB.ListScopeDefinitions(projectID)
    if err != nil {
        jsonError(w, "failed to load scope", http.StatusInternalServerError)
        return
    }
    
    definitions := make([]string, len(scopeRules))
    for i, rule := range scopeRules {
        definitions[i] = rule.Definition
    }
    
    matcher, err := scope.NewMatcher(definitions)
    if err != nil {
        jsonError(w, "invalid scope configuration", http.StatusInternalServerError)
        return
    }
    
    // Import with scope evaluation
    stats, err := importer.ImportObservations(s.DB, matcher, projectID, header.Filename, observations, time.Now().UTC())
    if err != nil {
        jsonError(w, "import failed: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success":        true,
        "filename":       header.Filename,
        "hosts_imported": stats.HostsFound,
        "ports_imported": stats.PortsFound,
        "hosts_in_scope": stats.InScope,
        "hosts_out_scope": stats.OutScope,
    })
}
```

Note: You may need to add a `ParseXMLBytes` function to the importer, or modify `ParseXMLFile` to accept an `io.Reader`.

### Frontend: Import Section on Dashboard

Add to `project.html`, near the top or in the header area:

```html
<div class="card" style="margin-top: 24px;">
  <div class="card-header">
    <h3 class="card-title">Import Nmap Scan</h3>
  </div>
  <div class="import-section">
    <p class="text-muted">Upload an Nmap XML file to import hosts and ports into this project.</p>
    
    <div class="import-dropzone" id="import-dropzone">
      <input type="file" id="import-file" accept=".xml" style="display: none;" onchange="handleFileSelect(event)">
      <div class="dropzone-content">
        <span class="dropzone-icon">📄</span>
        <p>Drag & drop an XML file here, or <a href="#" onclick="document.getElementById('import-file').click(); return false;">browse</a></p>
      </div>
    </div>
    
    <div id="import-status" class="import-status" style="display: none;">
      <span id="import-filename"></span>
      <button class="btn btn-primary" onclick="uploadFile()">Import</button>
      <button class="btn btn-secondary" onclick="clearImport()">Cancel</button>
    </div>
    
    <div id="import-progress" class="import-progress" style="display: none;">
      <span>Importing...</span>
    </div>
  </div>
</div>
```

CSS:
```css
.import-section {
  padding-top: 16px;
}

.import-dropzone {
  border: 2px dashed var(--border);
  border-radius: var(--radius-lg);
  padding: 40px;
  text-align: center;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.import-dropzone:hover,
.import-dropzone.drag-over {
  border-color: var(--accent);
  background: var(--accent-muted);
}

.dropzone-content {
  color: var(--text-muted);
}

.dropzone-icon {
  font-size: 32px;
  display: block;
  margin-bottom: 12px;
}

.dropzone-content a {
  color: var(--accent);
}

.import-status {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background: var(--bg-base);
  border-radius: var(--radius);
  margin-top: 16px;
}

.import-status span {
  flex: 1;
  font-family: var(--font-mono);
  font-size: 14px;
}

.import-progress {
  padding: 16px;
  text-align: center;
  color: var(--text-muted);
}
```

JavaScript:
```javascript
let selectedFile = null;

// Drag and drop handlers
const dropzone = document.getElementById('import-dropzone');

dropzone.addEventListener('dragover', (e) => {
  e.preventDefault();
  dropzone.classList.add('drag-over');
});

dropzone.addEventListener('dragleave', () => {
  dropzone.classList.remove('drag-over');
});

dropzone.addEventListener('drop', (e) => {
  e.preventDefault();
  dropzone.classList.remove('drag-over');
  
  const files = e.dataTransfer.files;
  if (files.length > 0) {
    handleFile(files[0]);
  }
});

dropzone.addEventListener('click', () => {
  document.getElementById('import-file').click();
});

function handleFileSelect(event) {
  const files = event.target.files;
  if (files.length > 0) {
    handleFile(files[0]);
  }
}

function handleFile(file) {
  if (!file.name.endsWith('.xml')) {
    showToast('Please select an XML file', 'error');
    return;
  }
  
  selectedFile = file;
  document.getElementById('import-filename').textContent = file.name;
  document.getElementById('import-dropzone').style.display = 'none';
  document.getElementById('import-status').style.display = 'flex';
}

function clearImport() {
  selectedFile = null;
  document.getElementById('import-file').value = '';
  document.getElementById('import-dropzone').style.display = 'block';
  document.getElementById('import-status').style.display = 'none';
}

async function uploadFile() {
  if (!selectedFile) return;
  
  const projectId = getProjectId();
  const formData = new FormData();
  formData.append('file', selectedFile);
  
  document.getElementById('import-status').style.display = 'none';
  document.getElementById('import-progress').style.display = 'block';
  
  try {
    const response = await fetch(`/api/projects/${projectId}/import`, {
      method: 'POST',
      body: formData
    });
    
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || 'Import failed');
    }
    
    const result = await response.json();
    
    showToast(
      `Imported ${result.hosts_imported} hosts, ${result.ports_imported} ports (${result.hosts_in_scope} in scope)`,
      'success'
    );
    
    // Reset and reload stats
    clearImport();
    document.getElementById('import-progress').style.display = 'none';
    document.getElementById('import-dropzone').style.display = 'block';
    loadDashboardStats();
    
  } catch (err) {
    showToast('Import failed: ' + err.message, 'error');
    document.getElementById('import-progress').style.display = 'none';
    document.getElementById('import-status').style.display = 'flex';
  }
}
```

---

## Feature 3: UI Fixes for Expandable Content

### Problem

1. Notes textareas can be resized to break the table layout
2. Script output can overflow its container
3. No easy way to copy script output content

### Solution: Constrained Expandable Containers

#### Script Output

Replace the current expandable script output with a fixed-height container that has:
- Max height with scroll by default
- Expand button to show full content
- Collapse button to return to default
- Copy button

```html
<div class="script-output-container">
  <div class="script-output-header">
    <span class="script-output-label">Script Output</span>
    <div class="script-output-actions">
      <button class="btn-icon" onclick="copyScriptOutput(this)" title="Copy">📋</button>
      <button class="btn-icon expand-btn" onclick="toggleScriptExpand(this)" title="Expand">⤢</button>
    </div>
  </div>
  <pre class="script-output-content">ssh-hostkey: 
  1024 ac:00:a0:1a:82:ff:cc:55:99:dc:67:2b:34:97:6b:75 (DSA)
  2048 20:3d:2d:44:62:2a:b0:5a:9d:b5:b3:05:14:c2:a6:b2 (RSA)
  ...</pre>
</div>
```

CSS:
```css
.script-output-container {
  background: var(--bg-base);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  overflow: hidden;
}

.script-output-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 12px;
  background: var(--bg-elevated);
  border-bottom: 1px solid var(--border);
}

.script-output-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
}

.script-output-actions {
  display: flex;
  gap: 4px;
}

.btn-icon {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 4px;
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  color: var(--text-muted);
  transition: all 0.15s;
}

.btn-icon:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.script-output-content {
  margin: 0;
  padding: 12px;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.5;
  color: var(--text);
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 120px;
  overflow-y: auto;
  transition: max-height 0.2s ease;
}

.script-output-content.expanded {
  max-height: none;
}
```

JavaScript:
```javascript
function toggleScriptExpand(btn) {
  const container = btn.closest('.script-output-container');
  const content = container.querySelector('.script-output-content');
  const isExpanded = content.classList.toggle('expanded');
  btn.textContent = isExpanded ? '⤡' : '⤢';
  btn.title = isExpanded ? 'Collapse' : 'Expand';
}

function copyScriptOutput(btn) {
  const container = btn.closest('.script-output-container');
  const content = container.querySelector('.script-output-content');
  
  navigator.clipboard.writeText(content.textContent).then(() => {
    showToast('Copied to clipboard', 'success');
  }).catch(() => {
    showToast('Failed to copy', 'error');
  });
}
```

#### Notes Textarea

Constrain the textarea so it can't break layout:

```css
.notes-cell textarea {
  width: 100%;
  min-height: 60px;
  max-height: 150px;  /* Prevent infinite expansion */
  resize: vertical;
  font-family: var(--font-mono);
  font-size: 13px;
}

/* On save, reset to default height */
.notes-cell textarea.saved {
  height: auto;
  min-height: 60px;
}
```

JavaScript - reset on save:
```javascript
async function saveNotes(textarea, endpoint) {
  const notes = textarea.value;
  
  try {
    await api(endpoint, {
      method: 'PUT',
      body: JSON.stringify({ notes })
    });
    
    // Reset textarea height
    textarea.style.height = 'auto';
    textarea.classList.add('saved');
    setTimeout(() => textarea.classList.remove('saved'), 100);
    
    showToast('Notes saved', 'success');
  } catch (err) {
    showToast('Failed to save notes: ' + err.message, 'error');
  }
}
```

#### Alternative: Modal for Long Content

For very long script output, consider a "View Full" button that opens a modal:

```html
<div id="content-modal" class="modal" style="display: none;">
  <div class="modal-backdrop" onclick="closeModal()"></div>
  <div class="modal-content">
    <div class="modal-header">
      <h3 id="modal-title">Script Output</h3>
      <div class="modal-actions">
        <button class="btn btn-secondary" onclick="copyModalContent()">Copy</button>
        <button class="btn-icon" onclick="closeModal()">×</button>
      </div>
    </div>
    <pre id="modal-body" class="modal-body"></pre>
  </div>
</div>
```

CSS:
```css
.modal {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-backdrop {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.7);
}

.modal-content {
  position: relative;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  width: 90%;
  max-width: 800px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 16px;
}

.modal-body {
  padding: 20px;
  overflow: auto;
  flex: 1;
  margin: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  white-space: pre-wrap;
  word-break: break-all;
}
```

JavaScript:
```javascript
function openModal(title, content) {
  document.getElementById('modal-title').textContent = title;
  document.getElementById('modal-body').textContent = content;
  document.getElementById('content-modal').style.display = 'flex';
}

function closeModal() {
  document.getElementById('content-modal').style.display = 'none';
}

function copyModalContent() {
  const content = document.getElementById('modal-body').textContent;
  navigator.clipboard.writeText(content).then(() => {
    showToast('Copied to clipboard', 'success');
  });
}

// Close on Escape key
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeModal();
});
```

---

## Implementation Checklist

### Backend

1. [ ] Verify/create `scope_definition` table
2. [ ] Add DB methods:
   - [ ] `ListScopeDefinitions(projectID)`
   - [ ] `AddScopeDefinition(projectID, definition, type)`
   - [ ] `DeleteScopeDefinition(id)`
   - [ ] `BulkAddScopeDefinitions(projectID, definitions)`
3. [ ] Update `internal/scope/matcher.go` with new logic
4. [ ] Add `EvaluateAllHosts(projectID)` method to re-check scope
5. [ ] Add API endpoints:
   - [ ] `GET /api/projects/{id}/scope`
   - [ ] `POST /api/projects/{id}/scope`
   - [ ] `DELETE /api/projects/{id}/scope/{scopeId}`
   - [ ] `POST /api/projects/{id}/scope/evaluate`
   - [ ] `POST /api/projects/{id}/import`
6. [ ] Add `ParseXMLBytes` or refactor `ParseXMLFile` to accept `io.Reader`
7. [ ] Update import logic to use scope matcher

### Frontend

1. [ ] Add scope management section to `project.html`
2. [ ] Add import dropzone to `project.html`
3. [ ] Update script output display in `host.html`
4. [ ] Constrain notes textarea sizing
5. [ ] Add copy functionality for script output
6. [ ] Add modal component for full-content viewing
7. [ ] Add all supporting JavaScript functions
8. [ ] Test all interactions with toast feedback

### Testing

1. [ ] Add scope rules (IPs and CIDRs mixed)
2. [ ] Import XML file via UI
3. [ ] Verify hosts are correctly marked in/out of scope
4. [ ] Modify scope rules and re-evaluate
5. [ ] Verify script output expand/collapse/copy works
6. [ ] Verify notes save and reset behavior

---

## Notes for the Agent

- The scope matcher logic may already partially exist in `internal/scope/matcher.go` - review and extend rather than rewrite if possible
- The importer already has some scope-related parameters - trace through the existing code flow
- For the XML import, use `multipart/form-data` not JSON
- Toast notifications should already be implemented from the styling prompt - reuse that
- Keep the UI consistent with the dark theme from the styling prompt
- Test with both single IPs and CIDR ranges of various sizes (/8, /16, /24, /32)
