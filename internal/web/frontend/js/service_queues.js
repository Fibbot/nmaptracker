const serviceQueueState = {
    projectId: null,
    campaign: 'smb',
    page: 1,
    pageSize: 50,
    totalHosts: 0,
    expandedHosts: {}
};

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    serviceQueueState.projectId = projectId;
    const requestedCampaign = String(getParam('campaign') || '').toLowerCase();
    if (['smb', 'ldap', 'rdp', 'http'].includes(requestedCampaign)) {
        serviceQueueState.campaign = requestedCampaign;
    }

    try {
        const project = await api(`/projects/${projectId}`);
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

function bindServiceQueueEvents() {
    document.getElementById('refresh-service-btn').addEventListener('click', loadServiceQueue);

    document.querySelectorAll('.queue-campaign-btn').forEach(btn => {
        btn.addEventListener('click', async () => {
            const campaign = btn.dataset.campaign;
            if (!campaign || serviceQueueState.campaign === campaign) {
                return;
            }
            serviceQueueState.campaign = campaign;
            serviceQueueState.page = 1;
            serviceQueueState.expandedHosts = {};
            setActiveCampaignButton();
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
}

function setActiveCampaignButton() {
    document.querySelectorAll('.queue-campaign-btn').forEach(btn => {
        if (btn.dataset.campaign === serviceQueueState.campaign) {
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
    setActiveCampaignButton();

    const params = new URLSearchParams();
    params.set('campaign', serviceQueueState.campaign);
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

    const meta = document.getElementById('queue-meta');
    meta.textContent = `Campaign: ${(result && result.campaign) || serviceQueueState.campaign} | Total Hosts: ${serviceQueueState.totalHosts} | Generated: ${(result && result.generated_at) || '-'}`;

    const sourceImports = document.getElementById('service-source-imports');
    const sourceIDs = (result && result.source_import_ids) || [];
    sourceImports.textContent = sourceIDs.length
        ? `Source import IDs on this page: ${sourceIDs.join(', ')}`
        : 'Source import IDs on this page: none';

    const tbody = document.getElementById('service-queue-rows');
    tbody.innerHTML = '';

    if (!items.length) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 5;
        td.style.textAlign = 'center';
        td.textContent = 'No hosts match this campaign on the current page.';
        tr.appendChild(td);
        tbody.appendChild(tr);
    } else {
        items.forEach(item => {
            const expanded = !!serviceQueueState.expandedHosts[item.host_id];

            const tr = document.createElement('tr');

            const expandTd = document.createElement('td');
            const expandBtn = document.createElement('button');
            expandBtn.className = 'btn btn-secondary';
            expandBtn.style.padding = '4px 8px';
            expandBtn.style.fontSize = '12px';
            expandBtn.textContent = expanded ? '▼' : '▶';
            expandBtn.addEventListener('click', () => {
                serviceQueueState.expandedHosts[item.host_id] = !serviceQueueState.expandedHosts[item.host_id];
                renderServiceQueue(result);
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

            tr.appendChild(expandTd);
            tr.appendChild(hostTd);
            tr.appendChild(summaryTd);
            tr.appendChild(latestTd);
            tr.appendChild(linkTd);
            tbody.appendChild(tr);

            if (expanded) {
                const detailTr = document.createElement('tr');
                const detailTd = document.createElement('td');
                detailTd.colSpan = 5;
                detailTd.innerHTML = renderMatchingPortsTable(item.matching_ports || []);
                detailTr.appendChild(detailTd);
                tbody.appendChild(detailTr);
            }
        });
    }

    const totalPages = Math.max(1, Math.ceil(serviceQueueState.totalHosts / serviceQueueState.pageSize));
    document.getElementById('service-page-info').textContent = `Page ${serviceQueueState.page} of ${totalPages}`;
    document.getElementById('service-prev-btn').disabled = serviceQueueState.page <= 1;
    document.getElementById('service-next-btn').disabled = serviceQueueState.page >= totalPages;
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
