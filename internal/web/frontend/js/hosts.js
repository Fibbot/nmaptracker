let currentPage = 1;
const PAGE_SIZE = 50;
let currentTotal = 0;
const latestScanOptions = [
    { value: 'none', label: 'None' },
    { value: 'ping_sweep', label: 'Ping Sweep' },
    { value: 'top1k', label: 'Top1k' },
    { value: 'full_port', label: 'Full Port' }
];

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    try {
        const project = await api(`/projects/${projectId}`);
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;

        await loadHosts();

        document.getElementById('filter-form').addEventListener('submit', (e) => {
            e.preventDefault();
            currentPage = 1;
            loadHosts();
        });

        document.getElementById('reset-btn').addEventListener('click', () => {
            document.getElementById('filter-form').reset();
            currentPage = 1;
            loadHosts();
        });

        document.getElementById('prev-btn').addEventListener('click', () => {
            if (currentPage > 1) {
                currentPage--;
                loadHosts();
            }
        });

        document.getElementById('next-btn').addEventListener('click', () => {
            if (currentPage * PAGE_SIZE < currentTotal) {
                currentPage++;
                loadHosts();
            }
        });

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

async function loadHosts() {
    const projectId = getProjectId();
    const form = document.getElementById('filter-form');
    const formData = new FormData(form);
    const params = new URLSearchParams();

    params.append('page', currentPage);
    params.append('page_size', PAGE_SIZE);

    for (const [key, value] of formData.entries()) {
        if (value) params.append(key, value);
    }

    try {
        const data = await api(`/projects/${projectId}/hosts?${params.toString()}`);
        currentTotal = data.total;
        renderHosts(data.items, projectId);
        updatePagination();
    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
}

function renderHosts(hosts, projectId) {
    const tbody = document.getElementById('hosts-list');
    tbody.innerHTML = '';

    if (!hosts || hosts.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 7;
        td.style.textAlign = 'center';
        td.textContent = 'No hosts found';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    hosts.forEach(h => {
        const tr = document.createElement('tr');

        const tdIp = document.createElement('td');
        const link = document.createElement('a');
        link.href = `host.html?id=${projectId}&hostId=${h.ID}`;
        link.textContent = h.IPAddress;
        tdIp.appendChild(link);

        const tdHost = document.createElement('td');
        tdHost.textContent = h.Hostname || '-';

        const tdScope = document.createElement('td');
        const scopeBadge = document.createElement('span');
        scopeBadge.className = h.InScope ? 'badge badge-yes' : 'badge badge-no';
        scopeBadge.textContent = h.InScope ? 'YES' : 'NO';
        tdScope.appendChild(scopeBadge);

        const tdPorts = document.createElement('td');
        tdPorts.textContent = h.PortCount;

        const tdStatus = document.createElement('td');
        const divStatus = document.createElement('div');
        divStatus.className = 'status-summary';

        // Compact badges logic
        if (h.Scanned) divStatus.appendChild(buildMiniBadge(`S:${h.Scanned}`, 'rgba(100,116,139,0.2)', '#94a3b8'));
        if (h.Flagged) divStatus.appendChild(buildMiniBadge(`F:${h.Flagged}`, 'rgba(245,158,11,0.15)', '#fbbf24'));
        if (h.InProgress) divStatus.appendChild(buildMiniBadge(`IP:${h.InProgress}`, 'rgba(34,211,238,0.15)', '#22d3ee'));
        if (h.Done) divStatus.appendChild(buildMiniBadge(`D:${h.Done}`, 'rgba(34,197,94,0.15)', '#4ade80'));

        if (!divStatus.children.length) divStatus.textContent = '-';
        tdStatus.appendChild(divStatus);

        const tdLatestScan = document.createElement('td');
        const latestScanSelect = document.createElement('select');
        latestScanSelect.style.width = '100%';
        latestScanOptions.forEach(option => {
            const opt = document.createElement('option');
            opt.value = option.value;
            opt.textContent = option.label;
            latestScanSelect.appendChild(opt);
        });
        latestScanSelect.value = normalizeLatestScan(h.LatestScan);
        latestScanSelect.addEventListener('change', () => updateLatestScan(projectId, h.ID, latestScanSelect.value, h.IPAddress));
        tdLatestScan.appendChild(latestScanSelect);

        const tdActions = document.createElement('td');
        const delBtn = document.createElement('button');
        delBtn.textContent = '×';
        delBtn.className = 'delete-btn';
        delBtn.title = 'Delete Host';
        delBtn.onclick = (e) => {
            e.preventDefault();
            deleteHost(projectId, h.ID, h.IPAddress);
        };
        tdActions.appendChild(delBtn);

        tr.appendChild(tdIp);
        tr.appendChild(tdHost);
        tr.appendChild(tdScope);
        tr.appendChild(tdPorts);
        tr.appendChild(tdStatus);
        tr.appendChild(tdLatestScan);
        tr.appendChild(tdActions);

        tbody.appendChild(tr);
    });
}

function buildMiniBadge(text, background, color) {
    const span = document.createElement('span');
    span.className = 'mini-badge';
    span.style.background = background;
    span.style.color = color;
    span.textContent = text;
    return span;
}

function updatePagination() {
    document.getElementById('page-info').textContent = `Page ${currentPage} of ${Math.ceil(currentTotal / PAGE_SIZE) || 1}`;
    document.getElementById('prev-btn').disabled = currentPage <= 1;
    document.getElementById('next-btn').disabled = currentPage * PAGE_SIZE >= currentTotal;
}

async function deleteHost(projectId, hostId, ip) {
    if (!confirm(`Are you sure you want to delete host ${ip}?`)) return;

    try {
        await api(`/projects/${projectId}/hosts/${hostId}`, {
            method: 'DELETE'
        });
        showToast(`Host ${ip} deleted`, 'success');
        loadHosts();
    } catch (err) {
        showToast(err.message, 'error');
    }
}

async function updateLatestScan(projectId, hostId, latestScan, ip) {
    try {
        await api(`/projects/${projectId}/hosts/${hostId}/latest-scan`, {
            method: 'PUT',
            body: JSON.stringify({ latest_scan: latestScan })
        });
        showToast(`Updated latest scan for ${ip}`, 'success');
    } catch (err) {
        showToast(err.message, 'error');
        loadHosts();
    }
}

function normalizeLatestScan(value) {
    const normalized = String(value || '').toLowerCase().trim();
    if (normalized === '' || normalized === 'none') return 'none';
    if (normalized === 'ping' || normalized === 'ping_sweep') return 'ping_sweep';
    if (normalized === 'top1k' || normalized === 'top_1k' || normalized === 'top_1k_tcp') return 'top1k';
    if (normalized === 'full' || normalized === 'full_port' || normalized === 'all_tcp') return 'full_port';
    return 'none';
}

function showToast(msg, type = 'info') {
    // Check if global showToast exists (from app.js??), if not define simple fallback or assume it exists.
    // dashboard.js uses showToast. app.js likely defines it.
    // I noticed app.js was present in the dir list.
    if (window.showToast) {
        window.showToast(msg, type);
    } else {
        alert(msg);
    }
}
