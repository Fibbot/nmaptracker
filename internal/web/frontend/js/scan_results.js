let currentFilteredPorts = [];

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

        loadAllPorts(projectId);

        loadAllPorts(projectId);

        document.getElementById('refresh-btn').addEventListener('click', () => loadAllPorts(projectId));

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
                loadAllPorts(projectId);
            } catch (err) {
                showToast(err.message, 'error');
            }
        });

        document.getElementById('status-filter-select').addEventListener('change', () => {
            // Update URL param without reload
            const val = document.getElementById('status-filter-select').value;
            const url = new URL(window.location);
            if (val) {
                url.searchParams.set('status', val);
            } else {
                url.searchParams.delete('status');
            }
            window.history.pushState({}, '', url);
            document.getElementById('filter-status-display').textContent = val ? `Filtering by: ${val.toUpperCase().replace('_', ' ')}` : '';
            loadAllPorts(projectId);
        });

        // Port Filters: Trigger re-render on change
        // Port Filters: Trigger re-render on change
        const allCheckbox = document.getElementById('filter-all');
        const filters = document.querySelectorAll('.port-filter');

        // All Checkbox logic
        allCheckbox.addEventListener('change', () => {
            filters.forEach(cb => cb.checked = allCheckbox.checked);
            const ports = window.allProjectPorts || [];
            renderAllPorts(ports, projectId);
        });

        filters.forEach(cb => {
            cb.addEventListener('change', () => {
                const allChecked = Array.from(filters).every(c => c.checked);
                allCheckbox.checked = allChecked;

                const ports = window.allProjectPorts || [];
                renderAllPorts(ports, projectId);
            });
        });

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

async function loadAllPorts(projectId) {
    const status = document.getElementById('status-filter-select').value;
    const params = new URLSearchParams();
    if (status) params.append('status', status);

    try {
        // Fetch all ports (filtered by status)
        const ports = await api(`/projects/${projectId}/ports/all?${params.toString()}`);
        window.allProjectPorts = ports; // Store for client-side filtering
        renderAllPorts(ports, projectId);
    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
}

function renderAllPorts(ports, projectId) {
    const tbody = document.getElementById('ports-list');
    tbody.innerHTML = '';

    // Client-side filtering by Port State (checkboxes)
    const checkedStates = Array.from(document.querySelectorAll('.port-filter:checked')).map(cb => cb.value);
    // Create a set or map for fast lookup if needed, but array.includes is fine

    // Filter
    const filteredPorts = ports.filter(p => checkedStates.includes(p.State));
    currentFilteredPorts = filteredPorts;

    if (!filteredPorts || filteredPorts.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" style="text-align: center;">No ports found matching filters</td></tr>';
        return;
    }

    filteredPorts.forEach(p => {
        const tr = document.createElement('tr');

        // Host Link
        const hostUrl = `host.html?id=${projectId}&hostID=${p.HostID}`;
        const hostDisplay = `<div><a href="${hostUrl}" style="font-weight:600; text-decoration:none; color:var(--primary-color);">${p.HostIP}</a></div><div style="font-size:12px; color:var(--text-dim);">${p.Hostname || ''}</div>`;

        tr.innerHTML = `
            <td>${hostDisplay}</td>
            <td>${p.PortNumber}/${p.Protocol}</td>
            <td><span class="badge badge-${escapeHtml(p.State)}">${p.State}</span></td>
            <td>
                <div style="font-weight: 500">${p.Service}</div>
                <div style="font-size: 12px; color: var(--text-dim);">${p.Product || ''} ${p.Version || ''}</div>
                ${p.ScriptOutput ? `
                    <button class="btn-icon" style="font-size: 12px;" onclick="openModal('Script Output', '${escapeHtml(p.ScriptOutput.replace(/'/g, "\\'"))}')">Script Output</button>
                ` : ''}
            </td>
        `;

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

        // Notes (ReadOnly-ish or simple edit?) Let's make it editable
        const tdNotes = document.createElement('td');
        const textarea = document.createElement('textarea');
        textarea.value = p.Notes || '';
        textarea.placeholder = "Notes...";

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

    makeSortable(document.querySelector('table'));
}

// Global Modal (Copied from host.js, ideally shared but duplication is acceptable for now)
function openModal(title, content) {
    document.getElementById('modal-title').textContent = title;
    document.getElementById('modal-body').textContent = content;
    document.getElementById('content-modal').style.display = 'flex';
}
function closeModal() {
    document.getElementById('content-modal').style.display = 'none';
}
function copyModalContent() {
    const content = document.getElementById('modal-body').textContent;
    navigator.clipboard.writeText(content).then(() => {
        showToast('Copied to clipboard', 'success');
    });
}
window.openModal = openModal;
window.closeModal = closeModal;
window.copyModalContent = copyModalContent;
