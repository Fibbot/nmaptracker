let currentFilteredPorts = [];
let currentPage = 1;
const PAGE_SIZE = 100;
let currentTotal = 0;

const ALLOWED_STATES = ['open', 'open|filtered', 'closed', 'filtered', 'closed|filtered', 'unfiltered'];

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

        // Initial Filter from URL
        const urlParams = new URLSearchParams(window.location.search);
        const initialStatus = urlParams.get('status');
        if (initialStatus) {
            document.getElementById('status-filter-select').value = initialStatus;
            document.getElementById('filter-status-display').textContent = `Filtering by: ${initialStatus.toUpperCase().replace('_', ' ')}`;
        }

        document.getElementById('bulk-done-btn').addEventListener('click', async () => {
            if (currentFilteredPorts.length === 0) {
                showToast('No ports to update', 'info');
                return;
            }
            if (!confirm(`Are you sure you want to mark ${currentFilteredPorts.length} ports as DONE?`)) return;

            const ids = currentFilteredPorts.map(p => p.ID);
            try {
                await api(`/projects/${projectId}/ports/bulk-status`, {
                    method: 'POST',
                    body: JSON.stringify({ ids: ids, status: 'done' })
                });
                showToast('Ports updated', 'success');
                loadPortsPage(projectId);
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        document.getElementById('status-filter-select').addEventListener('change', () => {
            const val = document.getElementById('status-filter-select').value;
            const url = new URL(window.location);
            if (val) {
                url.searchParams.set('status', val);
            } else {
                url.searchParams.delete('status');
            }
            window.history.pushState({}, '', url);
            document.getElementById('filter-status-display').textContent = val ? `Filtering by: ${val.toUpperCase().replace('_', ' ')}` : '';
            currentPage = 1;
            loadPortsPage(projectId);
        });

        const allCheckbox = document.getElementById('filter-all');
        const filters = document.querySelectorAll('.port-filter');

        allCheckbox.addEventListener('change', () => {
            filters.forEach(cb => cb.checked = allCheckbox.checked);
            currentPage = 1;
            loadPortsPage(projectId);
        });

        filters.forEach(cb => {
            cb.addEventListener('change', () => {
                const allChecked = Array.from(filters).every(c => c.checked);
                allCheckbox.checked = allChecked;
                currentPage = 1;
                loadPortsPage(projectId);
            });
        });

        document.getElementById('prev-btn').addEventListener('click', () => {
            if (currentPage > 1) {
                currentPage--;
                loadPortsPage(projectId);
            }
        });

        document.getElementById('next-btn').addEventListener('click', () => {
            if (currentPage * PAGE_SIZE < currentTotal) {
                currentPage++;
                loadPortsPage(projectId);
            }
        });

        makeSortable(document.querySelector('table'));
        loadPortsPage(projectId);

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

function getCheckedStates() {
    const checked = Array.from(document.querySelectorAll('.port-filter:checked')).map(cb => cb.value);
    return checked.filter(val => ALLOWED_STATES.includes(val));
}

async function loadPortsPage(projectId) {
    const status = document.getElementById('status-filter-select').value;
    const params = new URLSearchParams();
    params.append('page', currentPage);
    params.append('page_size', PAGE_SIZE);

    if (status) params.append('status', status);

    const states = getCheckedStates();
    if (states.length > 0) {
        params.append('state', states.join(','));
    }

    try {
        const data = await api(`/projects/${projectId}/ports/all?${params.toString()}`);
        currentTotal = data.total;
        renderPorts(data.items, projectId);
        updatePagination();
    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
}

function renderPorts(ports, projectId) {
    const tbody = document.getElementById('ports-list');
    tbody.innerHTML = '';

    currentFilteredPorts = ports || [];

    if (!ports || ports.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 6;
        td.style.textAlign = 'center';
        td.textContent = 'No ports found matching filters';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    ports.forEach(p => {
        const tr = document.createElement('tr');

        // Host Link
        const tdHost = document.createElement('td');
        const hostLink = document.createElement('a');
        hostLink.href = `host.html?id=${projectId}&hostId=${p.HostID}`;
        hostLink.style.fontWeight = '600';
        hostLink.style.textDecoration = 'none';
        hostLink.style.color = 'var(--primary-color)';
        hostLink.textContent = p.HostIP;
        const hostMeta = document.createElement('div');
        hostMeta.style.fontSize = '12px';
        hostMeta.style.color = 'var(--text-dim)';
        hostMeta.textContent = p.Hostname || '';
        tdHost.appendChild(hostLink);
        tdHost.appendChild(hostMeta);
        tr.appendChild(tdHost);

        const tdPort = document.createElement('td');
        tdPort.textContent = `${p.PortNumber}/${p.Protocol}`;
        tr.appendChild(tdPort);

        const tdState = document.createElement('td');
        const badge = document.createElement('span');
        badge.className = `badge ${stateBadgeClass(p.State)}`;
        badge.textContent = p.State;
        tdState.appendChild(badge);
        tr.appendChild(tdState);

        const tdService = document.createElement('td');
        const svcName = document.createElement('div');
        svcName.style.fontWeight = '500';
        svcName.textContent = p.Service || '';
        const svcMeta = document.createElement('div');
        svcMeta.style.fontSize = '12px';
        svcMeta.style.color = 'var(--text-dim)';
        svcMeta.textContent = `${p.Product || ''} ${p.Version || ''}`.trim();
        tdService.appendChild(svcName);
        tdService.appendChild(svcMeta);
        if (p.ScriptOutput) {
            const btn = document.createElement('button');
            btn.className = 'btn-icon';
            btn.style.fontSize = '12px';
            btn.textContent = 'Script Output';
            btn.addEventListener('click', () => {
                openModal('Script Output', p.ScriptOutput);
            });
            tdService.appendChild(btn);
        }
        tr.appendChild(tdService);

        const tdStatus = document.createElement('td');
        const select = document.createElement('select');
        select.style.width = '100%';
        ['scanned', 'flagged', 'in_progress', 'done', 'parking_lot'].forEach(s => {
            const opt = document.createElement('option');
            opt.value = s;
            opt.textContent = s.replace('_', ' ').toUpperCase();
            if (s === p.WorkStatus) opt.selected = true;
            select.appendChild(opt);
        });
        select.addEventListener('change', async () => {
            try {
                await api(`/projects/${projectId}/hosts/${p.HostID}/ports/${p.ID}/status`, {
                    method: 'PUT',
                    body: JSON.stringify({ status: select.value })
                });
                showToast('Status updated', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        });
        tdStatus.appendChild(select);
        tr.appendChild(tdStatus);

        const tdNotes = document.createElement('td');
        const textarea = document.createElement('textarea');
        textarea.value = p.Notes || '';
        textarea.placeholder = 'Notes...';

        let lastNotes = p.Notes || '';
        const saveNotes = async () => {
            if (textarea.value === lastNotes) return;
            try {
                await api(`/projects/${projectId}/hosts/${p.HostID}/ports/${p.ID}/notes`, {
                    method: 'PUT',
                    body: JSON.stringify({ notes: textarea.value })
                });
                lastNotes = textarea.value;
                textarea.classList.add('saved');
                setTimeout(() => textarea.classList.remove('saved'), 100);
                showToast('Notes saved', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        };
        const debouncedSave = debounce(saveNotes, 1000);
        textarea.addEventListener('input', debouncedSave);
        textarea.addEventListener('blur', () => { debouncedSave.cancel(); saveNotes(); });

        tdNotes.appendChild(textarea);
        tr.appendChild(tdNotes);

        tbody.appendChild(tr);
    });
}

function updatePagination() {
    document.getElementById('page-info').textContent = `Page ${currentPage} of ${Math.ceil(currentTotal / PAGE_SIZE) || 1}`;
    document.getElementById('prev-btn').disabled = currentPage <= 1;
    document.getElementById('next-btn').disabled = currentPage * PAGE_SIZE >= currentTotal;
}

function stateBadgeClass(state) {
    switch ((state || '').toLowerCase()) {
        case 'open':
            return 'badge-open';
        case 'closed':
            return 'badge-closed';
        case 'filtered':
        case 'open|filtered':
        case 'closed|filtered':
        case 'unfiltered':
            return 'badge-filtered';
        default:
            return 'badge-filtered';
    }
}
