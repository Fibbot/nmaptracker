document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    try {
        const [project, stats] = await Promise.all([
            api(`/projects/${projectId}`),
            api(`/projects/${projectId}/stats`)
        ]);

        // Set Headers
        document.title = `NmapTracker - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('project-title').textContent = project.Name;

        // Links
        document.getElementById('view-hosts-btn').href = `hosts.html?id=${projectId}`;
        document.getElementById('view-all-scans-btn').href = `scan_results.html?id=${projectId}`;
        document.getElementById('view-coverage-matrix-btn').href = `coverage_matrix.html?id=${projectId}`;
        document.getElementById('link-total-hosts').href = `hosts.html?id=${projectId}`;
        document.getElementById('link-in-scope').href = `hosts.html?id=${projectId}&in_scope=true`;
        document.getElementById('link-out-scope').href = `hosts.html?id=${projectId}&in_scope=false`;

        // Workflow Links
        document.getElementById('link-wf-scanned').href = `scan_results.html?id=${projectId}&status=scanned`;
        document.getElementById('link-wf-flagged').href = `scan_results.html?id=${projectId}&status=flagged`;
        document.getElementById('link-wf-in-progress').href = `scan_results.html?id=${projectId}&status=in_progress`;
        document.getElementById('link-wf-done').href = `scan_results.html?id=${projectId}&status=done`;

        // Export Links
        const exportMenu = document.getElementById('export-menu');
        exportMenu.innerHTML = `
            <a href="/api/projects/${projectId}/export?format=json" target="_blank" class="dropdown-item">JSON</a>
            <a href="/api/projects/${projectId}/export?format=csv" target="_blank" class="dropdown-item">CSV</a>
            <a href="/api/projects/${projectId}/export?format=text" target="_blank" class="dropdown-item">TXT</a>
        `;

        // Close dropdown when clicking outside
        document.addEventListener('click', (e) => {
            if (!e.target.closest('.dropdown')) {
                document.getElementById('export-menu').style.display = 'none';
            }
        });

        // Stats
        renderStats(stats);

        // Load Scope Rules
        loadScopeRules();

        // Setup Import Listeners
        setupImport();

        // Gap Dashboard
        loadGapDashboard();

    } catch (err) {
        console.error(err);
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

function renderStats(stats) {
    document.getElementById('stat-total').textContent = stats.TotalHosts;
    document.getElementById('stat-in-scope').textContent = stats.InScopeHosts;
    document.getElementById('stat-out-scope').textContent = stats.OutScopeHosts;

    document.getElementById('stat-scanned').textContent = stats.WorkStatus.Scanned;
    document.getElementById('stat-flagged').textContent = stats.WorkStatus.Flagged;
    document.getElementById('stat-in-progress').textContent = stats.WorkStatus.InProgress;
    document.getElementById('stat-done').textContent = stats.WorkStatus.Done;

    if (stats.InScopeHosts > 0) {
        const totalPorts = stats.WorkStatus.Scanned + stats.WorkStatus.Flagged + stats.WorkStatus.InProgress + stats.WorkStatus.Done;
        let pct = 0;
        if (totalPorts > 0) {
            pct = Math.round((stats.WorkStatus.Done / totalPorts) * 100);
        }
        const pctStr = `${pct}%`;
        document.getElementById('progress-percent').textContent = pctStr;
        document.getElementById('progress-fill').style.width = pctStr;
    } else {
        document.getElementById('progress-percent').textContent = 'N/A';
        document.getElementById('progress-fill').style.width = '0%';
    }
}

async function loadDashboardStats() {
    const projectId = getProjectId();
    try {
        const stats = await api(`/projects/${projectId}/stats`);
        renderStats(stats);
    } catch (err) {
        console.error("Failed to refresh stats", err);
    }
}

async function loadGapDashboard() {
    const projectId = getProjectId();
    if (!projectId) return;

    // Drill-down links
    document.getElementById('gap-link-never-scanned').href = `hosts.html?id=${projectId}&in_scope=true`;
    document.getElementById('gap-link-open-ports').href = `scan_results.html?id=${projectId}&status=scanned`;
    document.getElementById('gap-link-needs-ping').href = `hosts.html?id=${projectId}&in_scope=true`;
    document.getElementById('gap-link-needs-top1k').href = `hosts.html?id=${projectId}&in_scope=true`;
    document.getElementById('gap-link-needs-all').href = `hosts.html?id=${projectId}&in_scope=true`;

    try {
        const gaps = await api(`/projects/${projectId}/gaps?preview_size=10&include_lists=true`);
        renderGapDashboard(gaps);
    } catch (err) {
        console.error('Failed to load gap dashboard', err);
        showToast(`Gap dashboard failed: ${err.message}`, 'error');
    }
}

function renderGapDashboard(gaps) {
    const summary = gaps && gaps.summary ? gaps.summary : {};
    document.getElementById('gap-count-never-scanned').textContent = summary.in_scope_never_scanned || 0;
    document.getElementById('gap-count-open-ports').textContent = summary.open_ports_scanned_or_flagged || 0;
    document.getElementById('gap-count-needs-ping').textContent = summary.needs_ping_sweep || 0;
    document.getElementById('gap-count-needs-top1k').textContent = summary.needs_top_1k_tcp || 0;
    document.getElementById('gap-count-needs-all').textContent = summary.needs_all_tcp || 0;

    const lists = gaps && gaps.lists ? gaps.lists : {};
    renderGapHostList('gap-list-never-scanned', lists.in_scope_never_scanned, item => `${item.ip_address}${item.hostname ? ` (${item.hostname})` : ''}`);
    renderGapHostList('gap-list-needs-ping', lists.needs_ping_sweep, item => `${item.ip_address}${item.hostname ? ` (${item.hostname})` : ''}`);
    renderGapHostList('gap-list-needs-top1k', lists.needs_top_1k_tcp, item => `${item.ip_address}${item.hostname ? ` (${item.hostname})` : ''}`);
    renderGapHostList('gap-list-needs-all', lists.needs_all_tcp, item => `${item.ip_address}${item.hostname ? ` (${item.hostname})` : ''}`);
    renderGapHostList('gap-list-open-ports', lists.open_ports_scanned_or_flagged, item => `${item.ip_address} ${item.port_number}/${item.protocol} [${item.work_status}]`);
}

function renderGapHostList(listId, items, formatter) {
    const list = document.getElementById(listId);
    if (!list) return;

    list.innerHTML = '';
    if (!items || items.length === 0) {
        const empty = document.createElement('li');
        empty.className = 'text-muted';
        empty.textContent = 'None';
        list.appendChild(empty);
        return;
    }

    items.forEach(item => {
        const li = document.createElement('li');
        li.textContent = formatter(item);
        list.appendChild(li);
    });
}

// Scope Functions
async function loadScopeRules() {
    const projectId = getProjectId();
    const rules = await api(`/projects/${projectId}/scope`);
    renderScopeRules(rules);
}

function renderScopeRules(rules) {
    const list = document.getElementById('scope-rules-list');
    const empty = document.getElementById('scope-empty');

    if (!rules || rules.length === 0) {
        list.innerHTML = '';
        empty.style.display = 'block';
        return;
    }

    empty.style.display = 'none';
    list.innerHTML = '';
    rules.forEach(rule => {
        const item = document.createElement('li');
        item.className = 'scope-rule-item';

        const textWrap = document.createElement('span');
        const defText = document.createElement('span');
        defText.textContent = rule.Definition;
        const typeText = document.createElement('span');
        typeText.className = 'rule-type';
        typeText.textContent = rule.Type;
        textWrap.appendChild(defText);
        textWrap.appendChild(typeText);

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'delete-btn';
        deleteBtn.title = 'Remove';
        deleteBtn.textContent = '×';
        deleteBtn.addEventListener('click', () => deleteScopeRule(rule.ID));

        item.appendChild(textWrap);
        item.appendChild(deleteBtn);
        list.appendChild(item);
    });
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
        renderScopeRules(result.rules);
        // Refresh stats? Maybe re-eval is needed? 
        // Plan says explicit re-eval button.
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
        loadDashboardStats();
    } catch (err) {
        showToast('Failed to re-evaluate scope: ' + err.message, 'error');
    }
}

function toggleScope() {
    const content = document.getElementById('scope-content');
    const icon = document.getElementById('scope-toggle-icon');
    if (content.style.display === 'none') {
        content.style.display = 'block';
        icon.textContent = '▲';
    } else {
        content.style.display = 'none';
        icon.textContent = '▼';
    }
}

// Import Functions
let selectedFiles = [];

function setupImport() {
    const dropzone = document.getElementById('import-dropzone');
    const fileInput = document.getElementById('import-file');

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
            handleFiles(files);
        }
    });

    fileInput.addEventListener('change', (e) => {
        if (e.target.files.length > 0) {
            handleFiles(e.target.files);
        }
    });
}

function handleFiles(files) {
    let validFiles = [];
    for (let i = 0; i < files.length; i++) {
        if (files[i].name.endsWith('.xml')) {
            validFiles.push(files[i]);
        }
    }

    if (validFiles.length === 0) {
        showToast('Please select at least one XML file', 'error');
        return;
    }

    selectedFiles = validFiles;

    // Update UI
    const fileCount = selectedFiles.length;
    const countText = fileCount === 1 ? selectedFiles[0].name : `${fileCount} files selected`;

    document.getElementById('import-filename').textContent = countText;
    document.getElementById('import-dropzone').style.display = 'none';
    document.getElementById('import-status').style.display = 'flex';
}

function clearImport() {
    selectedFiles = [];
    document.getElementById('import-file').value = '';
    document.getElementById('import-dropzone').style.display = 'block';
    document.getElementById('import-status').style.display = 'none';
}

async function uploadFile() {
    if (selectedFiles.length === 0) return;

    const projectId = getProjectId();
    document.getElementById('import-status').style.display = 'none';
    document.getElementById('import-progress').style.display = 'block';

    let totalHosts = 0;
    let totalPorts = 0;
    let errors = [];

    for (let i = 0; i < selectedFiles.length; i++) {
        const file = selectedFiles[i];

        // Update progress text
        const statusText = document.getElementById('import-progress').querySelector('p');
        if (statusText) {
            statusText.textContent = `Importing ${i + 1} of ${selectedFiles.length}...`;
        }

        const formData = new FormData();
        formData.append('file', file);

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
            totalHosts += result.hosts_imported;
            totalPorts += result.ports_imported;

        } catch (err) {
            console.error(`Failed to import ${file.name}:`, err);
            errors.push(`${file.name}: ${err.message}`);
        }
    }

    // Finished
    document.getElementById('import-progress').style.display = 'none';

    if (errors.length > 0) {
        // Show mixed result or error
        if (totalHosts > 0) {
            showToast(`Imported ${totalHosts} hosts, ${totalPorts} ports. Failures: ${errors.join(', ')}`, 'warning');
        } else {
            showToast(`Import failed. Errors: ${errors.join(', ')}`, 'error');
            document.getElementById('import-status').style.display = 'flex';
            return; // Keep files selected to retry? Or clear? 
            // Let's keep them so user can retry or clear manually.
            // But UI is hidden. Let's show status again.
        }
    } else {
        showToast(`Successfully imported ${totalHosts} hosts, ${totalPorts} ports from ${selectedFiles.length} files.`, 'success');
    }

    if (errors.length === 0 || totalHosts > 0) {
        clearImport();
        document.getElementById('import-dropzone').style.display = 'block';
        loadDashboardStats();
    }
}

// Rename Project
// Rename Project
function toggleEditName() {
    const titleContainer = document.getElementById('page-title-container');
    const form = document.getElementById('rename-form');

    if (form.style.display === 'none') {
        // Show form
        titleContainer.style.display = 'none';
        form.style.display = 'flex';
        const currentName = document.getElementById('project-title').textContent;
        document.getElementById('rename-input').value = currentName;
        document.getElementById('rename-input').focus();
    } else {
        cancelRename();
    }
}

function cancelRename() {
    document.getElementById('page-title-container').style.display = 'block';
    document.getElementById('rename-form').style.display = 'none';
}

async function saveProjectName() {
    const projectId = getProjectId();
    const newName = document.getElementById('rename-input').value.trim();
    if (!newName) return;

    try {
        await api(`/projects/${projectId}`, {
            method: 'PUT',
            body: JSON.stringify({ name: newName })
        });
        document.getElementById('project-title').textContent = newName;
        document.getElementById('nav-project-name').textContent = newName;
        document.title = `NmapTracker - ${newName}`;
        cancelRename();
        showToast('Project renamed', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function toggleExportMenu() {
    const menu = document.getElementById('export-menu');
    menu.style.display = menu.style.display === 'block' ? 'none' : 'block';
}
window.toggleExportMenu = toggleExportMenu;
window.toggleEditName = toggleEditName;
window.saveProjectName = saveProjectName;
window.cancelRename = cancelRename;
window.loadGapDashboard = loadGapDashboard;
