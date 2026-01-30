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
        meta.innerHTML = `
            <span class="badge ${host.InScope ? 'badge-yes' : 'badge-no'}">${host.InScope ? 'In Scope' : 'Out of Scope'}</span>
            <span style="color: var(--text-muted); margin-left: 10px;">${host.OSGuess || 'OS Unknown'}</span>
        `;

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
        tbody.innerHTML = '<tr><td colspan="5" style="text-align: center;">No ports found matching filters</td></tr>';
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
                ${p.ScriptOutput ? `
                <div class="script-output-container">
                    <div class="script-output-header" onclick="this.closest('.script-output-container').classList.toggle('open');" style="cursor:pointer;">
                        <div class="flex-row">
                             <span class="toggle-icon" style="font-size: 10px; color: var(--text-dim); margin-right: 6px;">▶</span>
                             <span class="script-output-label">Script Output</span>
                        </div>
                        <div class="script-output-actions" onclick="event.stopPropagation();">
                            <button class="btn-icon" onclick="openModal('Script Output', this.closest('.script-output-container').querySelector('.script-output-content').textContent)" title="View Full">⤢</button>
                            <button class="btn-icon" onclick="navigator.clipboard.writeText(this.closest('.script-output-container').querySelector('.script-output-content').textContent); showToast('Copied', 'success')" title="Copy">📋</button>
                        </div>
                    </div>
                    <div class="script-output-body" style="display:none;">
                        <pre class="script-output-content">${escapeHtml(p.ScriptOutput)}</pre>
                    </div>
                </div>
                ` : ''}
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

// Global Modal Functions
function openModal(title, content) {
    document.getElementById('modal-title').textContent = title;
    document.getElementById('modal-body').textContent = content; // Using textContent safe against XSS if already raw
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
