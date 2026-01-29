# NmapTracker Frontend Styling: Terminal/Cyberpunk Aesthetic

## Objective

Transform the current functional-but-plain frontend into a polished, dark terminal-inspired UI. The functionality is already there and working - this is purely visual enhancement plus a few UX improvements (sortable tables, toast notifications).

## Design Direction

**Aesthetic**: Dark mode, terminal/cyberpunk inspired. Think hacker tools, not corporate SaaS.

**Color Palette**:
- **Background**: Near-black (`#0a0a0f` or similar)
- **Surface/Cards**: Dark purple-gray (`#1a1a2e`, `#16162a`)
- **Borders**: Subtle purple (`#2d2d44`, `#3d3d5c`)
- **Primary accent**: Purple/violet (`#8b5cf6`, `#a855f7`)
- **Secondary accent**: Cyan for contrast (`#22d3ee` or `#06b6d4`)
- **Text primary**: Light gray (`#e2e8f0`, `#f1f5f9`)
- **Text muted**: Medium gray (`#94a3b8`, `#64748b`)
- **Success**: Green (`#22c55e`)
- **Warning**: Amber (`#f59e0b`)
- **Danger**: Red (`#ef4444`)
- **Info**: Cyan (`#06b6d4`)

**Typography**:
- Monospace font for data/tables: `'JetBrains Mono', 'Fira Code', 'Consolas', monospace`
- Clean sans-serif for headings/UI: `'Inter', 'SF Pro', system-ui, sans-serif`

## Global Styling

### Base
```css
:root {
  --bg-base: #0a0a0f;
  --bg-surface: #1a1a2e;
  --bg-elevated: #242438;
  --border: #2d2d44;
  --border-hover: #3d3d5c;
  
  --accent: #8b5cf6;
  --accent-hover: #a855f7;
  --accent-muted: rgba(139, 92, 246, 0.2);
  
  --cyan: #22d3ee;
  --green: #22c55e;
  --amber: #f59e0b;
  --red: #ef4444;
  
  --text: #e2e8f0;
  --text-muted: #94a3b8;
  --text-dim: #64748b;
  
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace;
  --font-sans: 'Inter', system-ui, sans-serif;
  
  --radius: 8px;
  --radius-lg: 12px;
}

* {
  box-sizing: border-box;
}

body {
  background: var(--bg-base);
  color: var(--text);
  font-family: var(--font-sans);
  margin: 0;
  padding: 0;
  min-height: 100vh;
}
```

### Container/Layout
```css
.container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 24px 32px;
}

/* Optional: subtle grid/scan-line effect on background */
body::before {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: 
    linear-gradient(rgba(139, 92, 246, 0.03) 1px, transparent 1px);
  background-size: 100% 4px;
  pointer-events: none;
  z-index: -1;
}
```

## Component Specifications

### Navigation / Breadcrumbs

Current: Plain blue links with dots between them.

Target: Styled breadcrumb bar with better visual hierarchy.

```css
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  font-family: var(--font-mono);
  font-size: 13px;
  margin-bottom: 24px;
}

.breadcrumb a {
  color: var(--accent);
  text-decoration: none;
  transition: color 0.15s;
}

.breadcrumb a:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}

.breadcrumb .separator {
  color: var(--text-dim);
}

.breadcrumb .current {
  color: var(--text-muted);
}
```

Separator: Use `›` or `/` instead of `•`

### Cards / Surfaces

```css
.card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
  margin: 0;
}
```

### Page Headers

```css
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 32px;
}

.page-title {
  font-size: 28px;
  font-weight: 700;
  color: var(--text);
  margin: 0;
}

/* Optional glow effect on title */
.page-title::before {
  content: '> ';
  color: var(--accent);
}
```

### Buttons

```css
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 18px;
  font-family: var(--font-sans);
  font-size: 14px;
  font-weight: 500;
  border-radius: var(--radius);
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-primary {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.btn-primary:hover {
  background: var(--accent-hover);
  box-shadow: 0 0 20px rgba(139, 92, 246, 0.4);
}

.btn-secondary {
  background: transparent;
  color: var(--text);
  border-color: var(--border);
}

.btn-secondary:hover {
  border-color: var(--accent);
  color: var(--accent);
}

.btn-danger {
  background: var(--red);
  color: white;
  border-color: var(--red);
}

.btn-danger:hover {
  background: #dc2626;
  box-shadow: 0 0 20px rgba(239, 68, 68, 0.4);
}
```

### Tables (with sortable headers)

```css
.table-container {
  overflow-x: auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
}

table {
  width: 100%;
  border-collapse: collapse;
  font-family: var(--font-mono);
  font-size: 14px;
}

thead {
  background: var(--bg-elevated);
}

th {
  padding: 14px 16px;
  text-align: left;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  font-size: 11px;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--border);
  user-select: none;
}

/* Sortable header styling */
th.sortable {
  cursor: pointer;
  transition: color 0.15s;
}

th.sortable:hover {
  color: var(--accent);
}

th.sortable::after {
  content: ' ↕';
  opacity: 0.3;
  font-size: 10px;
}

th.sorted-asc::after {
  content: ' ↑';
  opacity: 1;
  color: var(--accent);
}

th.sorted-desc::after {
  content: ' ↓';
  opacity: 1;
  color: var(--accent);
}

td {
  padding: 14px 16px;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}

tr:last-child td {
  border-bottom: none;
}

/* Row hover - subtle */
tbody tr {
  transition: background 0.1s;
}

tbody tr:hover {
  background: rgba(139, 92, 246, 0.05);
}

/* Links in tables */
td a {
  color: var(--cyan);
  text-decoration: none;
}

td a:hover {
  text-decoration: underline;
}
```

### Sortable Table JavaScript

Add client-side sorting to all tables. When a sortable header is clicked:
1. Toggle sort direction (asc → desc → asc)
2. Sort the table rows in place
3. Update the header classes

```javascript
// In app.js or a dedicated sort.js

function makeSortable(table) {
  const headers = table.querySelectorAll('th.sortable');
  const tbody = table.querySelector('tbody');
  
  headers.forEach((header, index) => {
    header.addEventListener('click', () => {
      const isAsc = header.classList.contains('sorted-asc');
      
      // Clear all sort classes
      headers.forEach(h => h.classList.remove('sorted-asc', 'sorted-desc'));
      
      // Set new sort direction
      header.classList.add(isAsc ? 'sorted-desc' : 'sorted-asc');
      
      // Sort rows
      const rows = Array.from(tbody.querySelectorAll('tr'));
      const direction = isAsc ? -1 : 1;
      
      rows.sort((a, b) => {
        const aVal = a.cells[index].textContent.trim();
        const bVal = b.cells[index].textContent.trim();
        
        // Try numeric sort first
        const aNum = parseFloat(aVal);
        const bNum = parseFloat(bVal);
        if (!isNaN(aNum) && !isNaN(bNum)) {
          return (aNum - bNum) * direction;
        }
        
        // Fall back to string sort
        return aVal.localeCompare(bVal) * direction;
      });
      
      // Re-append sorted rows
      rows.forEach(row => tbody.appendChild(row));
    });
  });
}

// Call on page load for each table
document.querySelectorAll('table').forEach(makeSortable);
```

### Status Badges

```css
.badge {
  display: inline-block;
  padding: 4px 10px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-radius: 4px;
  font-family: var(--font-mono);
}

.badge-scanned {
  background: rgba(100, 116, 139, 0.2);
  color: #94a3b8;
  border: 1px solid rgba(100, 116, 139, 0.3);
}

.badge-flagged {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.badge-in-progress {
  background: rgba(34, 211, 238, 0.15);
  color: #22d3ee;
  border: 1px solid rgba(34, 211, 238, 0.3);
}

.badge-done {
  background: rgba(34, 197, 94, 0.15);
  color: #4ade80;
  border: 1px solid rgba(34, 197, 94, 0.3);
}

.badge-parking-lot {
  background: rgba(139, 92, 246, 0.15);
  color: #a78bfa;
  border: 1px solid rgba(139, 92, 246, 0.3);
}

/* For Yes/No scope badges */
.badge-yes {
  background: rgba(34, 197, 94, 0.15);
  color: #4ade80;
}

.badge-no {
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
}
```

### Progress Bar

```css
.progress-container {
  margin-top: 12px;
}

.progress-label {
  display: flex;
  justify-content: space-between;
  font-size: 13px;
  margin-bottom: 8px;
}

.progress-label span:first-child {
  color: var(--text-muted);
}

.progress-label span:last-child {
  color: var(--text);
  font-weight: 600;
  font-family: var(--font-mono);
}

.progress-bar {
  height: 8px;
  background: var(--bg-elevated);
  border-radius: 4px;
  overflow: hidden;
  border: 1px solid var(--border);
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent), var(--cyan));
  border-radius: 4px;
  transition: width 0.3s ease;
}
```

Usage in HTML:
```html
<div class="progress-container">
  <div class="progress-label">
    <span>Workflow Progress</span>
    <span>75%</span>
  </div>
  <div class="progress-bar">
    <div class="progress-fill" style="width: 75%"></div>
  </div>
</div>
```

### Toast Notifications

```css
.toast-container {
  position: fixed;
  top: 24px;
  right: 24px;
  z-index: 1000;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.toast {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 20px;
  background: var(--bg-elevated);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.5);
  font-size: 14px;
  animation: slideIn 0.2s ease;
  min-width: 280px;
}

.toast-success {
  border-left: 3px solid var(--green);
}

.toast-error {
  border-left: 3px solid var(--red);
}

.toast-info {
  border-left: 3px solid var(--cyan);
}

@keyframes slideIn {
  from {
    transform: translateX(100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}

@keyframes slideOut {
  from {
    transform: translateX(0);
    opacity: 1;
  }
  to {
    transform: translateX(100%);
    opacity: 0;
  }
}

.toast.removing {
  animation: slideOut 0.2s ease forwards;
}
```

Toast JavaScript:
```javascript
function showToast(message, type = 'info', duration = 3000) {
  const container = document.getElementById('toast-container') 
    || createToastContainer();
  
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  
  container.appendChild(toast);
  
  setTimeout(() => {
    toast.classList.add('removing');
    setTimeout(() => toast.remove(), 200);
  }, duration);
}

function createToastContainer() {
  const container = document.createElement('div');
  container.id = 'toast-container';
  container.className = 'toast-container';
  document.body.appendChild(container);
  return container;
}

// Usage:
// showToast('Notes saved successfully', 'success');
// showToast('Failed to update status', 'error');
```

### Form Elements

```css
input, select, textarea {
  background: var(--bg-base);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: 10px 14px;
  color: var(--text);
  font-family: var(--font-mono);
  font-size: 14px;
  transition: border-color 0.15s, box-shadow 0.15s;
}

input:focus, select:focus, textarea:focus {
  outline: none;
  border-color: var(--accent);
  box-shadow: 0 0 0 3px var(--accent-muted);
}

input::placeholder, textarea::placeholder {
  color: var(--text-dim);
}

select {
  cursor: pointer;
}

textarea {
  resize: vertical;
  min-height: 80px;
}

label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  margin-bottom: 6px;
}
```

### Filter Bar (for hosts page)

```css
.filter-bar {
  display: flex;
  gap: 12px;
  align-items: flex-end;
  flex-wrap: wrap;
  padding: 20px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  margin-bottom: 24px;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.filter-actions {
  display: flex;
  gap: 8px;
  margin-left: auto;
}
```

### Stats Grid (for dashboard)

Current: Awkward nested tables. 

Target: Clean stat cards in a grid.

```css
.stats-section {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 24px;
}

.stats-card {
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 24px;
}

.stats-card-title {
  font-size: 14px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border);
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.stat-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  background: var(--bg-base);
  border-radius: var(--radius);
  border: 1px solid var(--border);
}

.stat-label {
  font-size: 13px;
  color: var(--text-muted);
}

.stat-value {
  font-family: var(--font-mono);
  font-size: 18px;
  font-weight: 700;
  color: var(--text);
}

/* Make stats clickable where appropriate */
.stat-item.clickable {
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}

.stat-item.clickable:hover {
  border-color: var(--accent);
  background: var(--accent-muted);
}
```

Dashboard HTML structure:
```html
<div class="stats-section">
  <!-- Hosts Card -->
  <div class="stats-card">
    <div class="stats-card-title">Hosts</div>
    <div class="stats-grid">
      <a href="/hosts.html?id=1" class="stat-item clickable">
        <span class="stat-label">Total</span>
        <span class="stat-value">42</span>
      </a>
      <a href="/hosts.html?id=1&in_scope=true" class="stat-item clickable">
        <span class="stat-label">In Scope</span>
        <span class="stat-value">38</span>
      </a>
      <a href="/hosts.html?id=1&in_scope=false" class="stat-item clickable">
        <span class="stat-label">Out of Scope</span>
        <span class="stat-value">4</span>
      </a>
    </div>
  </div>

  <!-- Workflow Card -->
  <div class="stats-card">
    <div class="stats-card-title">Workflow</div>
    <div class="stats-grid">
      <div class="stat-item">
        <span class="stat-label">Scanned</span>
        <span class="stat-value">120</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">Flagged</span>
        <span class="stat-value">15</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">In Progress</span>
        <span class="stat-value">8</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">Done</span>
        <span class="stat-value">45</span>
      </div>
      <div class="stat-item">
        <span class="stat-label">Parking Lot</span>
        <span class="stat-value">3</span>
      </div>
    </div>
    
    <!-- Progress bar below the grid -->
    <div class="progress-container" style="margin-top: 20px;">
      <div class="progress-label">
        <span>Completion</span>
        <span>63%</span>
      </div>
      <div class="progress-bar">
        <div class="progress-fill" style="width: 63%"></div>
      </div>
    </div>
  </div>
</div>
```

### Status Summary in Host List

Current: `Scanned: 1, Done: 1` as plain text.

Target: Compact badge-style display.

```css
.status-summary {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.status-summary .mini-badge {
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 3px;
  font-family: var(--font-mono);
  font-weight: 600;
}
```

Or a compact format: `S:1 F:0 IP:0 D:1 P:0` with color-coded letters.

### Pagination

```css
.pagination {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 16px;
  margin-top: 24px;
  padding-top: 24px;
  border-top: 1px solid var(--border);
}

.pagination-btn {
  padding: 8px 16px;
  background: var(--bg-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
}

.pagination-btn:hover:not(:disabled) {
  border-color: var(--accent);
  color: var(--accent);
}

.pagination-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.pagination-info {
  font-family: var(--font-mono);
  font-size: 13px;
  color: var(--text-muted);
}
```

## Page-Specific Updates

### index.html (Projects)

- Add `sortable` class to ID, Name, Created headers
- Use `.badge-danger` styling for Delete button
- Add "New Project" button with `.btn-primary`
- Wrap in proper card container

### project.html (Dashboard)

- Use new `.stats-section` layout for Hosts/Workflow
- Make Hosts stats clickable (link to filtered hosts view)
- Add progress bar for workflow completion
- Style export buttons as `.btn-secondary`

### hosts.html (Host List)

- Use `.filter-bar` component
- All table headers sortable
- Status Summary column uses colored mini-badges
- In Scope uses `.badge-yes` / `.badge-no`
- Styled pagination

### host.html (Host Detail)

- Already good per your feedback
- Add toast notifications on save actions
- Ensure port status dropdowns use proper styling

## Font Loading

Add to `<head>`:
```html
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;600;700&display=swap" rel="stylesheet">
```

## Optional Enhancements

### Glow effects on focus/active states
```css
.btn-primary:focus,
input:focus,
select:focus {
  box-shadow: 0 0 0 3px var(--accent-muted), 0 0 20px var(--accent-muted);
}
```

### Subtle scanline animation (very optional, might be too much)
```css
@keyframes scanline {
  0% { transform: translateY(-100%); }
  100% { transform: translateY(100vh); }
}

body::after {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 4px;
  background: linear-gradient(transparent, rgba(139, 92, 246, 0.1), transparent);
  animation: scanline 8s linear infinite;
  pointer-events: none;
  z-index: 9999;
}
```

### Terminal-style cursor blink on page title
```css
.page-title::after {
  content: '_';
  animation: blink 1s step-end infinite;
  color: var(--accent);
}

@keyframes blink {
  50% { opacity: 0; }
}
```

## Implementation Checklist

1. [ ] Update `style.css` with all CSS from this document
2. [ ] Add font imports to all HTML files
3. [ ] Update `index.html`:
   - [ ] Wrap content in proper structure (`.container`, `.card`)
   - [ ] Add sortable classes to table headers
   - [ ] Style buttons properly
4. [ ] Update `project.html`:
   - [ ] Implement new stats card layout
   - [ ] Add progress bar component
   - [ ] Make stats clickable where appropriate
5. [ ] Update `hosts.html`:
   - [ ] Style filter bar
   - [ ] Add sortable headers
   - [ ] Implement status badges
   - [ ] Style pagination
6. [ ] Update `host.html`:
   - [ ] Ensure consistent styling
   - [ ] Verify toast notifications work on save
7. [ ] Add `sort.js` or sorting logic to `app.js`
8. [ ] Add toast notification system to `app.js`
9. [ ] Test all pages for visual consistency

## Notes for the Agent

- The backend API and functionality are complete - this is purely frontend styling
- Don't change any API calls or data handling logic
- Keep the HTML structure simple - we're not adding a build step
- Test that sorting works correctly for numeric vs string columns
- Toasts should appear on successful saves and on errors
- If something looks off, err on the side of simpler/cleaner
