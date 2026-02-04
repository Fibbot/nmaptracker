let currentFilteredPorts = [];

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    const hostId = getHostId();

    if (!projectId || !hostId) {
        window.location.href = 'index.html';
        return;
    }

    // Modal Close
    const closeBtn = document.querySelector('.modal .close'); // if exists standard
    // We bind explicitly in HTML but good to have keyboard support
    document.addEventListener('keydown', (e) => {
        if (e.key === 'Escape') closeModal();
    });

    // Initial Load
    try {
        const [project, host] = await Promise.all([
            api(`/projects/${projectId}`),
            api(`/projects/${projectId}/hosts/${hostId}`)
        ]);

        // Navigation & Header
        document.title = `NmapTracker - ${host.IPAddress}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('nav-hosts-link').href = `hosts.html?id=${projectId}`;
        document.getElementById('nav-host-ip').textContent = host.IPAddress;

        document.getElementById('host-title').textContent = `${host.IPAddress} (${host.Hostname || 'No Hostname'})`;

        const meta = document.getElementById('host-meta');
        meta.textContent = '';
        const scopeBadge = document.createElement('span');
        scopeBadge.className = `badge ${host.InScope ? 'badge-yes' : 'badge-no'}`;
        scopeBadge.textContent = host.InScope ? 'In Scope' : 'Out of Scope';
        const osGuess = document.createElement('span');
        osGuess.style.color = 'var(--text-muted)';
        osGuess.style.marginLeft = '10px';
        osGuess.textContent = host.OSGuess || 'OS Unknown';
        meta.appendChild(scopeBadge);
        meta.appendChild(osGuess);

        document.getElementById('host-notes').value = host.Notes || '';
        let lastHostNotes = host.Notes || '';

        // Notes Auto-Saving
        const notesArea = document.getElementById('host-notes');
        const saveHostNotes = async () => {
            const notes = notesArea.value;
            if (notes === lastHostNotes) return;

            try {
                await api(`/projects/${projectId}/hosts/${hostId}/notes`, {
                    method: 'PUT',
                    body: JSON.stringify({ notes })
                });
                lastHostNotes = notes;
                showToast('Host notes saved', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        };

        const debouncedHostNotes = debounce(saveHostNotes, 1000);
        notesArea.addEventListener('input', debouncedHostNotes);
        notesArea.addEventListener('blur', () => {
            debouncedHostNotes.cancel();
            saveHostNotes();
        });

        // Bulk Actions
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
                loadPorts(projectId, hostId);
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        // Port Filters
        const allCheckbox = document.getElementById('filter-all');
        const filters = document.querySelectorAll('.port-filter');

        // All Checkbox Logic
        allCheckbox.addEventListener('change', () => {
            filters.forEach(cb => cb.checked = allCheckbox.checked);
            renderPorts(window.hostPorts || [], projectId, hostId);
        });

        filters.forEach(cb => {
            cb.addEventListener('change', () => {
                // If any unchecked, uncheck All. If all checked, check All
                const allChecked = Array.from(filters).every(c => c.checked);
                allCheckbox.checked = allChecked;
                renderPorts(window.hostPorts || [], projectId, hostId);
            });
        });

        // Load Ports
        loadPorts(projectId, hostId);

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

async function loadPorts(projectId, hostId) {
    // Fetch ALL ports for client-side filtering
    try {
        const ports = await api(`/projects/${projectId}/hosts/${hostId}/ports`);
        window.hostPorts = ports; // Store globally
        renderPorts(ports, projectId, hostId);
    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
}

function renderPorts(allPorts, projectId, hostId) {
    const tbody = document.getElementById('ports-list');
    tbody.innerHTML = '';

    const states = Array.from(document.querySelectorAll('.port-filter:checked')).map(cb => cb.value);
    const ports = allPorts.filter(p => states.includes(p.State));

    currentFilteredPorts = ports || [];

    if (!ports || ports.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 5;
        td.style.textAlign = 'center';
        td.textContent = 'No ports found matching filters';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    ports.forEach(p => {
        const tr = document.createElement('tr');

        // Port/Proto
        const tdPort = document.createElement('td');
        tdPort.textContent = `${p.PortNumber}/${p.Protocol}`;

        const tdState = document.createElement('td');
        const badge = document.createElement('span');
        badge.className = `badge ${stateBadgeClass(p.State)}`;
        badge.textContent = p.State;
        tdState.appendChild(badge);

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
            const container = document.createElement('div');
            container.className = 'script-output-container';

            const header = document.createElement('div');
            header.className = 'script-output-header';
            header.style.cursor = 'pointer';
            header.addEventListener('click', () => {
                container.classList.toggle('open');
            });

            const left = document.createElement('div');
            left.className = 'flex-row';
            const toggleIcon = document.createElement('span');
            toggleIcon.className = 'toggle-icon';
            toggleIcon.style.fontSize = '10px';
            toggleIcon.style.color = 'var(--text-dim)';
            toggleIcon.style.marginRight = '6px';
            toggleIcon.textContent = '▶';
            const label = document.createElement('span');
            label.className = 'script-output-label';
            label.textContent = 'Script Output';
            left.appendChild(toggleIcon);
            left.appendChild(label);

            const actions = document.createElement('div');
            actions.className = 'script-output-actions';

            const viewBtn = document.createElement('button');
            viewBtn.className = 'btn-icon';
            viewBtn.title = 'View Full';
            viewBtn.textContent = '⤢';
            viewBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                openModal('Script Output', p.ScriptOutput);
            });

            const copyBtn = document.createElement('button');
            copyBtn.className = 'btn-icon';
            copyBtn.title = 'Copy';
            copyBtn.textContent = '📋';
            copyBtn.addEventListener('click', (e) => {
                e.stopPropagation();
                navigator.clipboard.writeText(p.ScriptOutput).then(() => {
                    showToast('Copied', 'success');
                });
            });

            actions.appendChild(viewBtn);
            actions.appendChild(copyBtn);

            header.appendChild(left);
            header.appendChild(actions);

            const body = document.createElement('div');
            body.className = 'script-output-body';
            body.style.display = 'none';
            const pre = document.createElement('pre');
            pre.className = 'script-output-content';
            pre.textContent = p.ScriptOutput;
            body.appendChild(pre);

            container.appendChild(header);
            container.appendChild(body);
            tdService.appendChild(container);
        }

        tr.appendChild(tdPort);
        tr.appendChild(tdState);
        tr.appendChild(tdService);

        // Status Select
        const tdStatus = document.createElement('td');
        const select = document.createElement('select');
        select.style.width = '100%';
        ['scanned', 'flagged', 'in_progress', 'done'].forEach(s => {
            const opt = document.createElement('option');
            opt.value = s;
            opt.textContent = s.replace('_', ' ').toUpperCase();
            if (s === p.WorkStatus) opt.selected = true;
            select.appendChild(opt);
        });
        tdStatus.appendChild(select);
        tr.appendChild(tdStatus);

        // Notes - Constrained with saved indicator
        const tdNotes = document.createElement('td');
        tdNotes.className = 'notes-cell'; // for CSS targeting
        const textarea = document.createElement('textarea');
        textarea.value = p.Notes || '';
        textarea.placeholder = "Notes...";
        tdNotes.appendChild(textarea);
        tr.appendChild(tdNotes);

        tbody.appendChild(tr);

        // Event Listeners

        // 1. Status Auto-Save
        select.addEventListener('change', async () => {
            const newStatus = select.value;
            try {
                await api(`/projects/${projectId}/hosts/${hostId}/ports/${p.ID}/status`, {
                    method: 'PUT',
                    body: JSON.stringify({ status: newStatus })
                });
                showToast('Port status saved', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        // 2. Notes Auto-Save
        let lastPortNotes = p.Notes || '';

        const savePortNotes = async () => {
            const newNotes = textarea.value;
            if (newNotes === lastPortNotes) return;

            try {
                await api(`/projects/${projectId}/hosts/${hostId}/ports/${p.ID}/notes`, {
                    method: 'PUT',
                    body: JSON.stringify({ notes: newNotes })
                });

                lastPortNotes = newNotes;

                // Visual feedback
                textarea.classList.add('saved');
                setTimeout(() => textarea.classList.remove('saved'), 100);
                showToast('Port notes saved', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        };

        const debouncedPortNotes = debounce(savePortNotes, 1000);
        textarea.addEventListener('input', debouncedPortNotes);
        textarea.addEventListener('blur', () => {
            debouncedPortNotes.cancel();
            savePortNotes();
        });
    });
}

function toggleHostNotes() {
    const content = document.getElementById('host-notes-content');
    const icon = document.getElementById('host-notes-toggle-icon');
    if (content.style.display === 'none') {
        content.style.display = 'block';
        icon.textContent = '▲';
    } else {
        content.style.display = 'none';
        icon.textContent = '▼';
    }
}
window.toggleHostNotes = toggleHostNotes;

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
