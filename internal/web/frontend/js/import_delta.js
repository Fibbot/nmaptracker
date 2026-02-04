let importsCache = [];
let deltaResult = null;
let activeTab = 'net_new_hosts';

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    try {
        const project = await api(`/projects/${projectId}`);
        document.title = `NmapTracker - Import Delta - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('back-to-project').href = `project.html?id=${projectId}`;

        await loadImports(projectId);

        document.getElementById('compare-btn').addEventListener('click', () => compareSelectedImports(projectId));
        document.getElementById('export-delta-btn').addEventListener('click', exportDeltaJSON);
        document.querySelectorAll('.delta-tab').forEach(btn => {
            btn.addEventListener('click', () => {
                activeTab = btn.dataset.tab;
                renderActiveTab();
            });
        });

        setActiveTabButton();
    } catch (err) {
        showError(err.message);
    }
});

async function loadImports(projectId) {
    const payload = await api(`/projects/${projectId}/imports`);
    importsCache = (payload && payload.items) ? payload.items : [];

    const baseSelect = document.getElementById('base-import-select');
    const targetSelect = document.getElementById('target-import-select');
    baseSelect.innerHTML = '';
    targetSelect.innerHTML = '';

    if (importsCache.length < 2) {
        showError('At least two imports are required to compare delta.');
        document.getElementById('compare-btn').disabled = true;
        return;
    }

    importsCache.forEach(item => {
        const label = formatImportLabel(item);
        const baseOption = document.createElement('option');
        baseOption.value = item.id;
        baseOption.textContent = label;
        baseSelect.appendChild(baseOption);

        const targetOption = document.createElement('option');
        targetOption.value = item.id;
        targetOption.textContent = label;
        targetSelect.appendChild(targetOption);
    });

    // Default: target latest, base previous.
    targetSelect.value = String(importsCache[importsCache.length - 1].id);
    baseSelect.value = String(importsCache[importsCache.length - 2].id);

    hideError();
    await compareSelectedImports(projectId);
}

function formatImportLabel(item) {
    const dt = item.import_time ? new Date(item.import_time).toLocaleString() : 'unknown time';
    return `#${item.id} - ${item.filename || 'import'} (${dt})`;
}

async function compareSelectedImports(projectId) {
    const baseImportId = Number(document.getElementById('base-import-select').value);
    const targetImportId = Number(document.getElementById('target-import-select').value);

    if (!baseImportId || !targetImportId) {
        showError('Please select both imports.');
        return;
    }
    if (baseImportId === targetImportId) {
        showError('Base and target imports must be different.');
        return;
    }

    hideError();
    try {
        const params = new URLSearchParams({
            base_import_id: String(baseImportId),
            target_import_id: String(targetImportId),
            preview_size: '200',
            include_lists: 'true'
        });
        deltaResult = await api(`/projects/${projectId}/delta?${params.toString()}`);
        renderDelta(deltaResult);
        document.getElementById('export-delta-btn').disabled = false;
    } catch (err) {
        showError(err.message);
    }
}

function renderDelta(delta) {
    const summary = delta && delta.summary ? delta.summary : {};
    document.getElementById('sum-net-new-hosts').textContent = summary.net_new_hosts || 0;
    document.getElementById('sum-disappeared-hosts').textContent = summary.disappeared_hosts || 0;
    document.getElementById('sum-net-new-exposures').textContent = summary.net_new_open_exposures || 0;
    document.getElementById('sum-disappeared-exposures').textContent = summary.disappeared_open_exposures || 0;
    document.getElementById('sum-changed-fingerprints').textContent = summary.changed_service_fingerprints || 0;

    const baseLabel = delta.base_import ? `${delta.base_import.filename} (#${delta.base_import.id})` : '-';
    const targetLabel = delta.target_import ? `${delta.target_import.filename} (#${delta.target_import.id})` : '-';
    document.getElementById('delta-meta').textContent = `Base: ${baseLabel} | Target: ${targetLabel}`;

    renderActiveTab();
}

function renderActiveTab() {
    setActiveTabButton();

    const container = document.getElementById('delta-list-container');
    if (!deltaResult || !deltaResult.lists) {
        container.innerHTML = '<div style="padding: 16px;" class="text-muted">No comparison result yet.</div>';
        return;
    }

    const items = deltaResult.lists[activeTab] || [];
    switch (activeTab) {
        case 'net_new_hosts':
        case 'disappeared_hosts':
            renderHostTable(container, items);
            return;
        case 'net_new_open_exposures':
        case 'disappeared_open_exposures':
            renderExposureTable(container, items);
            return;
        case 'changed_service_fingerprints':
            renderFingerprintTable(container, items);
            return;
        default:
            container.innerHTML = '<div style="padding: 16px;" class="text-muted">No data.</div>';
    }
}

function renderHostTable(container, items) {
    if (!items.length) {
        container.innerHTML = '<div style="padding: 16px;" class="text-muted">No host changes in this category.</div>';
        return;
    }

    const rows = items.map(item => `<tr><td>${escapeHtml(item.ip_address)}</td><td>${escapeHtml(item.hostname || '-')}</td></tr>`).join('');
    container.innerHTML = `<table><thead><tr><th>IP Address</th><th>Hostname</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function renderExposureTable(container, items) {
    if (!items.length) {
        container.innerHTML = '<div style="padding: 16px;" class="text-muted">No exposure changes in this category.</div>';
        return;
    }

    const rows = items.map(item => `
        <tr>
            <td>${escapeHtml(item.ip_address)}</td>
            <td>${item.port_number}/${escapeHtml(item.protocol)}</td>
            <td>${escapeHtml(item.state || '-')}</td>
            <td>${escapeHtml(item.service || '-')}</td>
        </tr>`).join('');
    container.innerHTML = `<table><thead><tr><th>IP Address</th><th>Port</th><th>State</th><th>Service</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function renderFingerprintTable(container, items) {
    if (!items.length) {
        container.innerHTML = '<div style="padding: 16px;" class="text-muted">No fingerprint changes in this category.</div>';
        return;
    }

    const rows = items.map(item => {
        const before = item.before || {};
        const after = item.after || {};
        return `
            <tr>
                <td>${escapeHtml(item.ip_address)}</td>
                <td>${item.port_number}/${escapeHtml(item.protocol)}</td>
                <td>${escapeHtml(formatFingerprint(before))}</td>
                <td>${escapeHtml(formatFingerprint(after))}</td>
            </tr>`;
    }).join('');

    container.innerHTML = `<table><thead><tr><th>IP Address</th><th>Port</th><th>Before</th><th>After</th></tr></thead><tbody>${rows}</tbody></table>`;
}

function formatFingerprint(fp) {
    const service = fp.service || '-';
    const product = fp.product || '-';
    const version = fp.version || '-';
    const extra = fp.extra_info || '-';
    return `${service} | ${product} | ${version} | ${extra}`;
}

function setActiveTabButton() {
    document.querySelectorAll('.delta-tab').forEach(btn => {
        if (btn.dataset.tab === activeTab) {
            btn.classList.remove('btn-secondary');
            btn.classList.add('btn-primary');
        } else {
            btn.classList.add('btn-secondary');
            btn.classList.remove('btn-primary');
        }
    });
}

function exportDeltaJSON() {
    if (!deltaResult) return;
    const blob = new Blob([JSON.stringify(deltaResult, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `import-delta-${Date.now()}.json`;
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
}

function showError(message) {
    const el = document.getElementById('error-msg');
    el.textContent = message;
    el.style.display = 'block';
}

function hideError() {
    const el = document.getElementById('error-msg');
    el.textContent = '';
    el.style.display = 'none';
}
