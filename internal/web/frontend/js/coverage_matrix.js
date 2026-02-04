let matrixData = null;
let missingState = {
    projectId: null,
    segmentKey: '',
    segmentLabel: '',
    intent: '',
    page: 1,
    pageSize: 25,
    total: 0
};

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    missingState.projectId = projectId;

    try {
        const project = await api(`/projects/${projectId}`);
        document.title = `NmapTracker - Coverage Matrix - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('back-to-project').href = `project.html?id=${projectId}`;

        document.getElementById('refresh-btn').addEventListener('click', loadCoverageMatrix);

        document.getElementById('missing-prev-btn').addEventListener('click', () => {
            if (missingState.page > 1) {
                missingState.page--;
                loadMissingHosts();
            }
        });
        document.getElementById('missing-next-btn').addEventListener('click', () => {
            if (missingState.page * missingState.pageSize < missingState.total) {
                missingState.page++;
                loadMissingHosts();
            }
        });

        await loadCoverageMatrix();
    } catch (err) {
        showError(err.message);
    }
});

async function loadCoverageMatrix() {
    const projectId = getProjectId();
    const includePreview = document.getElementById('preview-toggle').checked;
    const previewSize = Number(document.getElementById('preview-size').value || 5);

    const params = new URLSearchParams();
    params.set('include_missing_preview', includePreview ? 'true' : 'false');
    params.set('missing_preview_size', String(previewSize));

    hideError();
    try {
        matrixData = await api(`/projects/${projectId}/coverage-matrix?${params.toString()}`);
        renderCoverageMatrix(matrixData);
    } catch (err) {
        showError(err.message);
    }
}

function renderCoverageMatrix(data) {
    const headerRow = document.getElementById('matrix-header-row');
    const tbody = document.getElementById('matrix-rows');
    const meta = document.getElementById('matrix-meta');

    headerRow.innerHTML = '<th style="min-width: 240px;">Segment</th>';
    (data.intents || []).forEach(intent => {
        const th = document.createElement('th');
        th.textContent = intentLabel(intent);
        headerRow.appendChild(th);
    });

    meta.textContent = `Mode: ${data.segment_mode || '-'} | Generated: ${data.generated_at || '-'}`;

    tbody.innerHTML = '';
    if (!data.segments || data.segments.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = Math.max(2, (data.intents || []).length + 1);
        td.style.textAlign = 'center';
        td.textContent = 'No in-scope hosts available for coverage segmentation.';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    data.segments.forEach(segment => {
        const tr = document.createElement('tr');

        const segTd = document.createElement('td');
        segTd.innerHTML = `<strong>${escapeHtml(segment.segment_label)}</strong><br><span class="text-muted">Hosts: ${segment.host_total}</span>`;
        tr.appendChild(segTd);

        (data.intents || []).forEach(intent => {
            const td = document.createElement('td');
            const cell = (segment.cells && segment.cells[intent]) || {
                covered_count: 0,
                missing_count: 0,
                coverage_percent: 0,
                missing_hosts: []
            };

            const wrap = document.createElement('div');
            wrap.className = 'flex-column';
            wrap.style.gap = '6px';

            const stat = document.createElement('div');
            stat.innerHTML = `<strong>${cell.coverage_percent}%</strong> <span class="text-muted">(${cell.covered_count}/${segment.host_total})</span>`;
            wrap.appendChild(stat);

            if (cell.missing_count > 0) {
                const btn = document.createElement('button');
                btn.className = 'btn btn-secondary';
                btn.style.padding = '4px 8px';
                btn.style.fontSize = '12px';
                btn.textContent = `Missing: ${cell.missing_count}`;
                btn.addEventListener('click', () => openMissingModal(segment, intent));
                wrap.appendChild(btn);

                if (cell.missing_hosts && cell.missing_hosts.length > 0) {
                    const preview = document.createElement('div');
                    preview.className = 'text-muted';
                    preview.style.fontSize = '12px';
                    preview.textContent = `Preview: ${cell.missing_hosts.map(h => h.ip_address).join(', ')}`;
                    wrap.appendChild(preview);
                }
            }

            td.appendChild(wrap);
            tr.appendChild(td);
        });

        tbody.appendChild(tr);
    });
}

function intentLabel(intent) {
    switch (intent) {
        case 'ping_sweep':
            return 'Ping Sweep';
        case 'top_1k_tcp':
            return 'Top 1k TCP';
        case 'all_tcp':
            return 'All TCP';
        case 'top_udp':
            return 'Top UDP';
        case 'vuln_nse':
            return 'Vuln NSE';
        default:
            return intent;
    }
}

function openMissingModal(segment, intent) {
    missingState.segmentKey = segment.segment_key;
    missingState.segmentLabel = segment.segment_label;
    missingState.intent = intent;
    missingState.page = 1;

    document.getElementById('missing-modal').style.display = 'flex';
    document.getElementById('missing-modal-title').textContent = `${intentLabel(intent)} Missing Hosts`;

    const hostsUrl = buildHostsLink(missingState.projectId, segment.segment_key, segment.segment_label);
    const link = document.getElementById('missing-hosts-link');
    link.href = hostsUrl;

    loadMissingHosts();
}

async function loadMissingHosts() {
    const params = new URLSearchParams();
    params.set('segment_key', missingState.segmentKey);
    params.set('intent', missingState.intent);
    params.set('page', String(missingState.page));
    params.set('page_size', String(missingState.pageSize));

    try {
        const result = await api(`/projects/${missingState.projectId}/coverage-matrix/missing?${params.toString()}`);
        missingState.total = result.total || 0;
        renderMissingHosts(result.items || []);
    } catch (err) {
        showToast(`Failed to load missing hosts: ${err.message}`, 'error');
    }
}

function renderMissingHosts(items) {
    const tbody = document.getElementById('missing-host-list');
    const summary = document.getElementById('missing-modal-summary');
    const pageInfo = document.getElementById('missing-page-info');
    const prevBtn = document.getElementById('missing-prev-btn');
    const nextBtn = document.getElementById('missing-next-btn');

    summary.textContent = `${missingState.segmentLabel} | ${intentLabel(missingState.intent)} | Missing ${missingState.total}`;

    tbody.innerHTML = '';
    if (!items || items.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 3;
        td.style.textAlign = 'center';
        td.textContent = 'No missing hosts found on this page.';
        tr.appendChild(td);
        tbody.appendChild(tr);
    } else {
        items.forEach(item => {
            const tr = document.createElement('tr');

            const ipTd = document.createElement('td');
            ipTd.textContent = item.ip_address;

            const hostTd = document.createElement('td');
            hostTd.textContent = item.hostname || '-';

            const viewTd = document.createElement('td');
            const a = document.createElement('a');
            a.className = 'btn btn-secondary';
            a.style.padding = '4px 8px';
            a.style.fontSize = '12px';
            a.href = `host.html?id=${missingState.projectId}&hostId=${item.host_id}`;
            a.textContent = 'View';
            viewTd.appendChild(a);

            tr.appendChild(ipTd);
            tr.appendChild(hostTd);
            tr.appendChild(viewTd);
            tbody.appendChild(tr);
        });
    }

    const totalPages = Math.max(1, Math.ceil(missingState.total / missingState.pageSize));
    pageInfo.textContent = `Page ${missingState.page} of ${totalPages}`;
    prevBtn.disabled = missingState.page <= 1;
    nextBtn.disabled = missingState.page >= totalPages;
}

function buildHostsLink(projectId, segmentKey, segmentLabel) {
    const params = new URLSearchParams();
    params.set('id', projectId);
    params.set('in_scope', 'true');

    const subnet = segmentSubnet(segmentKey, segmentLabel);
    if (subnet) {
        params.set('subnet', subnet);
    }

    return `hosts.html?${params.toString()}`;
}

function segmentSubnet(segmentKey, segmentLabel) {
    if (segmentKey === 'scope:unmapped') {
        return '';
    }
    if (segmentKey.startsWith('fallback:')) {
        return segmentLabel;
    }
    if (/^\d+\.\d+\.\d+\.\d+\/\d+$/.test(segmentLabel)) {
        return segmentLabel;
    }
    if (/^\d+\.\d+\.\d+\.\d+$/.test(segmentLabel)) {
        return `${segmentLabel}/32`;
    }
    return '';
}

function closeCoverageMissingModal() {
    document.getElementById('missing-modal').style.display = 'none';
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

window.closeCoverageMissingModal = closeCoverageMissingModal;
