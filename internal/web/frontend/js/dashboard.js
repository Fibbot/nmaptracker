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
        document.getElementById('link-total-hosts').href = `hosts.html?id=${projectId}`;
        document.getElementById('link-in-scope').href = `hosts.html?id=${projectId}&in_scope=true`;
        document.getElementById('link-out-scope').href = `hosts.html?id=${projectId}&in_scope=false`;

        // Export Links
        const exportDiv = document.getElementById('export-links');
        exportDiv.innerHTML = `
            <a href="/api/projects/${projectId}/export?format=json" target="_blank" class="btn btn-secondary">Export JSON</a>
            <a href="/api/projects/${projectId}/export?format=csv" target="_blank" class="btn btn-secondary">Export CSV</a>
        `;

        // Stats
        renderStats(stats);

        // Load Scope Rules
        loadScopeRules();

        // Setup Import Listeners
        setupImport();

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
    document.getElementById('stat-parking').textContent = stats.WorkStatus.ParkingLot;

    if (stats.InScopeHosts > 0) {
        const pct = Math.round((stats.WorkStatus.Done / stats.InScopeHosts) * 100);
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
    list.innerHTML = rules.map(rule => `
        <li class="scope-rule-item">
            <span>
                ${escapeHtml(rule.Definition)}
                <span class="rule-type">${rule.Type}</span>
            </span>
            <button class="delete-btn" onclick="deleteScopeRule(${rule.ID})" title="Remove">×</button>
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
