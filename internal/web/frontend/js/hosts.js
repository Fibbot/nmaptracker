let currentPage = 1;
const PAGE_SIZE = 50;
let currentTotal = 0;

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
        tr.innerHTML = '<td colspan="5" style="text-align: center;">No hosts found</td>';
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
        tdStatus.className = 'status-summary';

        // Compact badges logic
        if (h.Scanned) tdStatus.innerHTML += `<span class="mini-badge" style="background:rgba(100,116,139,0.2); color:#94a3b8">S:${h.Scanned}</span>`;
        if (h.Flagged) tdStatus.innerHTML += `<span class="mini-badge" style="background:rgba(245,158,11,0.15); color:#fbbf24">F:${h.Flagged}</span>`;
        if (h.InProgress) tdStatus.innerHTML += `<span class="mini-badge" style="background:rgba(34,211,238,0.15); color:#22d3ee">IP:${h.InProgress}</span>`;
        if (h.Done) tdStatus.innerHTML += `<span class="mini-badge" style="background:rgba(34,197,94,0.15); color:#4ade80">D:${h.Done}</span>`;
        if (h.ParkingLot) tdStatus.innerHTML += `<span class="mini-badge" style="background:rgba(139,92,246,0.15); color:#a78bfa">P:${h.ParkingLot}</span>`;
        if (!tdStatus.innerHTML) tdStatus.textContent = '-';

        tr.appendChild(tdIp);
        tr.appendChild(tdHost);
        tr.appendChild(tdScope);
        tr.appendChild(tdPorts);
        tr.appendChild(tdStatus);

        tbody.appendChild(tr);
    });
}

function updatePagination() {
    document.getElementById('page-info').textContent = `Page ${currentPage} of ${Math.ceil(currentTotal / PAGE_SIZE) || 1}`;
    document.getElementById('prev-btn').disabled = currentPage <= 1;
    document.getElementById('next-btn').disabled = currentPage * PAGE_SIZE >= currentTotal;
}
