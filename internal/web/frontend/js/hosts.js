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
        tdScope.textContent = h.InScope ? 'Yes' : 'No';
        if (h.InScope) tdScope.style.color = 'green';

        const tdPorts = document.createElement('td');
        tdPorts.textContent = h.PortCount;

        const tdStatus = document.createElement('td');
        // Simple summary string
        const parts = [];
        if (h.Scanned) parts.push(`Scanned: ${h.Scanned}`);
        if (h.Flagged) parts.push(`Flagged: ${h.Flagged}`);
        if (h.InProgress) parts.push(`In Progress: ${h.InProgress}`);
        if (h.Done) parts.push(`Done: ${h.Done}`);
        tdStatus.textContent = parts.join(', ') || '-';

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
