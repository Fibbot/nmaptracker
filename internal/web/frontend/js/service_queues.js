const CAMPAIGN_ORDER = ['http', 'ldap', 'rdp', 'smb', 'ssh'];
const CAMPAIGN_LABELS = {
    smb: 'SMB',
    ldap: 'LDAP',
    rdp: 'RDP',
    http: 'HTTP(S)',
    ssh: 'SSH'
};

const serviceQueueState = {
    projectId: null,
    projectName: '',
    campaigns: new Set(['smb']),
    page: 1,
    pageSize: 50,
    totalHosts: 0,
    expandedHosts: {},
    selectedHosts: {},
    currentPageHosts: []
};

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    serviceQueueState.projectId = projectId;

    const requestedCampaigns = parseCampaignsFromURL();
    if (requestedCampaigns.length > 0) {
        serviceQueueState.campaigns = new Set(requestedCampaigns);
    }

    try {
        const project = await api(`/projects/${projectId}`);
        serviceQueueState.projectName = project.Name || '';
        document.title = `NmapTracker - Service Queues - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('back-to-project').href = `project.html?id=${projectId}`;

        bindServiceQueueEvents();
        await loadServiceQueue();
    } catch (err) {
        showError(err.message);
    }
});

function parseCampaignsFromURL() {
    const params = new URLSearchParams(window.location.search);
    const tokens = [];
    params.getAll('campaign').forEach(raw => {
        String(raw || '').split(',').forEach(token => {
            tokens.push(token);
        });
    });
    return normalizeCampaignTokens(tokens);
}

function normalizeCampaignTokens(values) {
    const seen = new Set();
    const out = [];
    values.forEach(value => {
        const normalized = String(value || '').toLowerCase().trim();
        if (!normalized || !CAMPAIGN_LABELS[normalized] || seen.has(normalized)) {
            return;
        }
        seen.add(normalized);
        out.push(normalized);
    });
    out.sort((a, b) => CAMPAIGN_ORDER.indexOf(a) - CAMPAIGN_ORDER.indexOf(b));
    return out;
}

function getSelectedCampaigns() {
    const values = Array.from(serviceQueueState.campaigns);
    values.sort((a, b) => CAMPAIGN_ORDER.indexOf(a) - CAMPAIGN_ORDER.indexOf(b));
    return values;
}

function getCampaignTokenForFilename() {
    const values = Array.from(serviceQueueState.campaigns);
    values.sort();
    return values.join('-');
}

function sanitizeFilenamePart(value, fallback) {
    const cleaned = String(value || '')
        .trim()
        .replace(/[^A-Za-z0-9._-]+/g, '_')
        .replace(/^_+|_+$/g, '');
    return cleaned || fallback;
}

function bindServiceQueueEvents() {
    document.getElementById('refresh-service-btn').addEventListener('click', loadServiceQueue);

    document.querySelectorAll('.queue-campaign-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            const campaign = String(btn.dataset.campaign || '').toLowerCase().trim();
            if (!campaign || !CAMPAIGN_LABELS[campaign]) {
                return;
            }

            if (serviceQueueState.campaigns.has(campaign)) {
                if (serviceQueueState.campaigns.size === 1) {
                    showToast('At least one campaign filter is required.', 'info');
                    return;
                }
                serviceQueueState.campaigns.delete(campaign);
            } else {
                serviceQueueState.campaigns.add(campaign);
            }

            serviceQueueState.page = 1;
            serviceQueueState.expandedHosts = {};
            setActiveCampaignButtons();
            updateCampaignQueryParams();
            await loadServiceQueue();
        });
    });

    document.getElementById('service-prev-btn').addEventListener('click', async () => {
        if (serviceQueueState.page <= 1) return;
        serviceQueueState.page -= 1;
        await loadServiceQueue();
    });

    document.getElementById('service-next-btn').addEventListener('click', async () => {
        if (serviceQueueState.page * serviceQueueState.pageSize >= serviceQueueState.totalHosts) return;
        serviceQueueState.page += 1;
        await loadServiceQueue();
    });

    document.getElementById('service-select-all-visible').addEventListener('change', (event) => {
        const checked = !!event.target.checked;
        serviceQueueState.currentPageHosts.forEach(item => {
            if (checked) {
                serviceQueueState.selectedHosts[item.host_id] = item.ip_address;
            } else {
                delete serviceQueueState.selectedHosts[item.host_id];
            }
        });
        syncSelectionUI();
        renderServiceQueueRows(serviceQueueState.currentPageHosts);
    });

    document.getElementById('copy-selected-ips-btn').addEventListener('click', async () => {
        const ips = getSelectedIPs();
        if (!ips.length) {
            showToast('No hosts selected.', 'info');
            return;
        }
        const text = ips.join('\n');
        try {
            await navigator.clipboard.writeText(text);
            showToast(`Copied ${ips.length} IPs to clipboard.`, 'success');
        } catch (err) {
            showToast('Clipboard copy failed.', 'error');
        }
    });

    document.getElementById('export-selected-ips-btn').addEventListener('click', () => {
        const ips = getSelectedIPs();
        if (!ips.length) {
            showToast('No hosts selected.', 'info');
            return;
        }

        const projectToken = sanitizeFilenamePart(serviceQueueState.projectName, `project-${serviceQueueState.projectId}`);
        const filterToken = sanitizeFilenamePart(getCampaignTokenForFilename(), 'hosts');
        const filename = `${projectToken}_${filterToken}-hosts.txt`;

        const blob = new Blob([`${ips.join('\n')}\n`], { type: 'text/plain' });
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
    });
}

function getSelectedIPs() {
    return Object.values(serviceQueueState.selectedHosts)
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }));
}

function updateCampaignQueryParams() {
    const url = new URL(window.location.href);
    url.searchParams.delete('campaign');
    getSelectedCampaigns().forEach(campaign => url.searchParams.append('campaign', campaign));
    window.history.replaceState({}, '', url);
}

function setActiveCampaignButtons() {
    document.querySelectorAll('.queue-campaign-btn').forEach(btn => {
        const campaign = String(btn.dataset.campaign || '').toLowerCase().trim();
        if (serviceQueueState.campaigns.has(campaign)) {
            btn.classList.remove('btn-secondary');
            btn.classList.add('btn-primary');
        } else {
            btn.classList.remove('btn-primary');
            btn.classList.add('btn-secondary');
        }
    });
}

async function loadServiceQueue() {
    hideError();
    setActiveCampaignButtons();

    const params = new URLSearchParams();
    getSelectedCampaigns().forEach(campaign => params.append('campaign', campaign));
    params.set('page', String(serviceQueueState.page));
    params.set('page_size', String(serviceQueueState.pageSize));

    try {
        const result = await api(`/projects/${serviceQueueState.projectId}/queues/services?${params.toString()}`);
        renderServiceQueue(result);
    } catch (err) {
        showError(err.message);
    }
}

function renderServiceQueue(result) {
    const items = (result && result.items) || [];
    serviceQueueState.totalHosts = (result && result.total_hosts) || 0;
    serviceQueueState.currentPageHosts = items;

    const meta = document.getElementById('queue-meta');
    const campaignLabels = (((result && result.campaigns) || getSelectedCampaigns())
        .map(campaign => CAMPAIGN_LABELS[campaign] || campaign.toUpperCase())
        .join(', '));
    meta.textContent = `Campaigns: ${campaignLabels || '-'} | Total Hosts: ${serviceQueueState.totalHosts} | Generated: ${(result && result.generated_at) || '-'}`;

    const sourceImports = document.getElementById('service-source-imports');
    const sourceIDs = (result && result.source_import_ids) || [];
    sourceImports.textContent = sourceIDs.length
        ? `Source import IDs on this page: ${sourceIDs.join(', ')}`
        : 'Source import IDs on this page: none';

    renderServiceQueueRows(items);

    const totalPages = Math.max(1, Math.ceil(serviceQueueState.totalHosts / serviceQueueState.pageSize));
    document.getElementById('service-page-info').textContent = `Page ${serviceQueueState.page} of ${totalPages}`;
    document.getElementById('service-prev-btn').disabled = serviceQueueState.page <= 1;
    document.getElementById('service-next-btn').disabled = serviceQueueState.page >= totalPages;

    syncSelectionUI();
}

function renderServiceQueueRows(items) {
    const tbody = document.getElementById('service-queue-rows');
    tbody.innerHTML = '';

    if (!items.length) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 6;
        td.style.textAlign = 'center';
        td.textContent = 'No hosts match the selected campaigns on the current page.';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    items.forEach(item => {
        const expanded = !!serviceQueueState.expandedHosts[item.host_id];
        const selected = !!serviceQueueState.selectedHosts[item.host_id];

        const tr = document.createElement('tr');

        const selectTd = document.createElement('td');
        const select = document.createElement('input');
        select.type = 'checkbox';
        select.checked = selected;
        select.addEventListener('change', () => {
            if (select.checked) {
                serviceQueueState.selectedHosts[item.host_id] = item.ip_address;
            } else {
                delete serviceQueueState.selectedHosts[item.host_id];
            }
            syncSelectionUI();
        });
        selectTd.appendChild(select);

        const expandTd = document.createElement('td');
        const expandBtn = document.createElement('button');
        expandBtn.className = 'btn btn-secondary';
        expandBtn.style.padding = '4px 8px';
        expandBtn.style.fontSize = '12px';
        expandBtn.textContent = expanded ? '▼' : '▶';
        expandBtn.addEventListener('click', () => {
            serviceQueueState.expandedHosts[item.host_id] = !serviceQueueState.expandedHosts[item.host_id];
            renderServiceQueueRows(items);
        });
        expandTd.appendChild(expandBtn);

        const hostTd = document.createElement('td');
        hostTd.innerHTML = `<strong>${escapeHtml(item.ip_address)}</strong><br><span class="text-muted">${escapeHtml(item.hostname || '-')}</span>`;

        const summaryTd = document.createElement('td');
        summaryTd.innerHTML = renderStatusSummaryBadges(item.status_summary || {});

        const latestTd = document.createElement('td');
        latestTd.textContent = item.latest_seen ? new Date(item.latest_seen).toLocaleString() : '-';

        const linkTd = document.createElement('td');
        const link = document.createElement('a');
        link.className = 'btn btn-secondary';
        link.style.padding = '4px 8px';
        link.style.fontSize = '12px';
        link.href = `host.html?id=${serviceQueueState.projectId}&hostId=${item.host_id}`;
        link.textContent = 'View';
        linkTd.appendChild(link);

        tr.appendChild(selectTd);
        tr.appendChild(expandTd);
        tr.appendChild(hostTd);
        tr.appendChild(summaryTd);
        tr.appendChild(latestTd);
        tr.appendChild(linkTd);
        tbody.appendChild(tr);

        if (expanded) {
            const detailTr = document.createElement('tr');
            const detailTd = document.createElement('td');
            detailTd.colSpan = 6;
            detailTd.innerHTML = renderMatchingPortsTable(item.matching_ports || []);
            detailTr.appendChild(detailTd);
            tbody.appendChild(detailTr);
        }
    });
}

function syncSelectionUI() {
    const selectedCount = Object.keys(serviceQueueState.selectedHosts).length;
    document.getElementById('service-selected-count').textContent = `Selected: ${selectedCount}`;

    const selectAll = document.getElementById('service-select-all-visible');
    const visibleIDs = serviceQueueState.currentPageHosts.map(item => item.host_id);
    if (!visibleIDs.length) {
        selectAll.checked = false;
        selectAll.indeterminate = false;
        return;
    }

    const selectedVisible = visibleIDs.filter(hostID => !!serviceQueueState.selectedHosts[hostID]).length;
    selectAll.checked = selectedVisible === visibleIDs.length;
    selectAll.indeterminate = selectedVisible > 0 && selectedVisible < visibleIDs.length;
}

function renderStatusSummaryBadges(summary) {
    const parts = [
        renderSummaryBadge('scanned', summary.scanned || 0),
        renderSummaryBadge('flagged', summary.flagged || 0),
        renderSummaryBadge('in_progress', summary.in_progress || 0),
        renderSummaryBadge('done', summary.done || 0)
    ];
    return `<div class="status-summary">${parts.join(' ')}</div>`;
}

function renderSummaryBadge(status, count) {
    const label = status === 'in_progress' ? 'IN PROGRESS' : status.replace('_', ' ').toUpperCase();
    return `<span class="badge badge-${status}">${label}: ${count}</span>`;
}

function renderMatchingPortsTable(ports) {
    if (!ports.length) {
        return '<div class="text-muted" style="padding: 8px 0;">No matching ports for this host.</div>';
    }

    const rows = ports.map(port => {
        const statusClass = normalizeWorkStatus(port.work_status);
        return `
        <tr>
            <td>${port.port_number}/${escapeHtml(port.protocol)}</td>
            <td>${escapeHtml(port.state || '-')}</td>
            <td>${escapeHtml(port.service || '-')}</td>
            <td>${escapeHtml(port.product || '-')}</td>
            <td>${escapeHtml(port.version || '-')}</td>
            <td><span class="badge badge-${escapeHtml(statusClass)}">${escapeHtml(port.work_status || '-')}</span></td>
            <td>${port.last_seen ? new Date(port.last_seen).toLocaleString() : '-'}</td>
        </tr>
    `;
    }).join('');

    return `
        <div class="table-container" style="margin: 8px 0;">
            <table>
                <thead>
                    <tr>
                        <th>Port</th>
                        <th>State</th>
                        <th>Service</th>
                        <th>Product</th>
                        <th>Version</th>
                        <th>Status</th>
                        <th>Last Seen</th>
                    </tr>
                </thead>
                <tbody>${rows}</tbody>
            </table>
        </div>
    `;
}

function normalizeWorkStatus(status) {
    const normalized = String(status || '').toLowerCase().trim();
    if (['scanned', 'flagged', 'in_progress', 'done'].includes(normalized)) {
        return normalized;
    }
    return 'scanned';
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
