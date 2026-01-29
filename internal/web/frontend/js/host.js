document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    const hostId = getHostId();

    if (!projectId || !hostId) {
        window.location.href = 'index.html';
        return;
    }

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
        meta.innerHTML = `
            <span class="badge ${host.InScope ? 'badge-yes' : 'badge-no'}">${host.InScope ? 'In Scope' : 'Out of Scope'}</span>
            <span style="color: var(--text-muted); margin-left: 10px;">${host.OSGuess || 'OS Unknown'}</span>
        `;

        document.getElementById('host-notes').value = host.Notes || '';

        // Notes Saving
        document.getElementById('save-notes-btn').addEventListener('click', async () => {
            const notes = document.getElementById('host-notes').value;
            try {
                await api(`/projects/${projectId}/hosts/${hostId}/notes`, {
                    method: 'PUT',
                    body: JSON.stringify({ notes })
                });
                showToast('Host notes saved', 'success');
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        // Bulk Actions
        document.getElementById('bulk-done-btn').addEventListener('click', async () => {
            if (!confirm('Mark all open ports as DONE?')) return;
            try {
                await api(`/projects/${projectId}/hosts/${hostId}/bulk-status`, {
                    method: 'POST',
                    body: JSON.stringify({ status: 'done' })
                });
                showToast('All open ports marked as DONE', 'success');
                loadPorts(projectId, hostId);
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        // Port Filters
        document.querySelectorAll('.port-filter').forEach(cb => {
            cb.addEventListener('change', () => loadPorts(projectId, hostId));
        });

        document.getElementById('refresh-ports-btn').addEventListener('click', () => {
            loadPorts(projectId, hostId);
            showToast('Ports refreshed', 'info');
        });

        // Load Ports
        loadPorts(projectId, hostId);

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

async function loadPorts(projectId, hostId) {
    const states = Array.from(document.querySelectorAll('.port-filter:checked')).map(cb => cb.value);
    const params = new URLSearchParams();
    states.forEach(s => params.append('state', s));

    try {
        const ports = await api(`/projects/${projectId}/hosts/${hostId}/ports?${params.toString()}`);
        renderPorts(ports, projectId, hostId);
    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
}

function renderPorts(ports, projectId, hostId) {
    const tbody = document.getElementById('ports-list');
    tbody.innerHTML = '';

    if (!ports || ports.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center;">No ports found matching filters</td></tr>';
        return;
    }

    ports.forEach(p => {
        const tr = document.createElement('tr');

        // Port/Proto
        tr.innerHTML = `
            <td>${p.PortNumber}/${p.Protocol}</td>
            <td><span class="badge badge-${p.State}">${p.State}</span></td>
            <td>
                <div style="font-weight: 500">${p.Service}</div>
                <div style="font-size: 12px; color: var(--text-dim);">${p.Product || ''} ${p.Version || ''}</div>
                ${p.ScriptOutput ? `<details style="margin-top:4px;"><summary style="cursor:pointer;color:var(--accent);">Script Output</summary><pre style="font-size: 11px; background:var(--bg-base); padding:8px; border-radius:4px; overflow: auto; max-width: 400px; max-height: 200px;">${p.ScriptOutput}</pre></details>` : ''}
            </td>
        `;

        // Status Select
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
        tdStatus.appendChild(select);
        tr.appendChild(tdStatus);

        // Notes
        const tdNotes = document.createElement('td');
        const textarea = document.createElement('textarea');
        textarea.value = p.Notes || '';
        textarea.style.height = '60px';
        textarea.placeholder = "Notes...";
        tdNotes.appendChild(textarea);
        tr.appendChild(tdNotes);

        // Action
        const tdAction = document.createElement('td');
        const saveBtn = document.createElement('button');
        saveBtn.textContent = 'Save';
        saveBtn.className = 'btn btn-primary';
        saveBtn.style.padding = '6px 12px';
        saveBtn.style.width = '100%';
        saveBtn.style.fontSize = '12px';
        tdAction.appendChild(saveBtn);
        tr.appendChild(tdAction);

        tbody.appendChild(tr);

        // Event Listener for Save
        saveBtn.addEventListener('click', async () => {
            const newStatus = select.value;
            const newNotes = textarea.value;

            try {
                // Update Status
                if (newStatus !== p.WorkStatus) {
                    await api(`/projects/${projectId}/hosts/${hostId}/ports/${p.ID}/status`, {
                        method: 'PUT',
                        body: JSON.stringify({ status: newStatus })
                    });
                }

                // Update Notes
                await api(`/projects/${projectId}/hosts/${hostId}/ports/${p.ID}/notes`, {
                    method: 'PUT',
                    body: JSON.stringify({ notes: newNotes })
                });

                showToast('Port saved', 'success');
                // Could re-fetch to be safe, or just update local state if we had it.

            } catch (err) {
                showToast(err.message, 'error');
            }
        });
    });
}
