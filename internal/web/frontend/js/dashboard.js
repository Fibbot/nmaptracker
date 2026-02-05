const DASHBOARD_SECTION_ORDER = [
    'import_scan',
    'scope_definition',
    'import_intents',
    'hosts_workflow',
    'service_campaign_queues',
    'expected_asset_baseline'
];

const IMPORT_INTENT_OPTIONS = [
    { id: 'ping_sweep', label: 'Ping Sweep' },
    { id: 'top_1k_tcp', label: 'Top 1k TCP' },
    { id: 'all_tcp', label: 'All TCP' },
    { id: 'top_udp', label: 'Top UDP' },
    { id: 'vuln_nse', label: 'Vuln NSE' }
];

const baselineExpectedUnseenState = {
    allItems: [],
    query: '',
    page: 1,
    pageSize: 50
};

let importIntentsCache = [];

function dashboardLayoutStorageKey(projectId) {
    return `nmaptracker:project-dashboard-layout:v1:${projectId}`;
}

function loadDashboardLayout(projectId) {
    try {
        const raw = localStorage.getItem(dashboardLayoutStorageKey(projectId));
        if (!raw) {
            return { order: DASHBOARD_SECTION_ORDER.slice(), collapsed: {} };
        }
        const parsed = JSON.parse(raw);
        const order = Array.isArray(parsed.order) ? parsed.order.filter(id => typeof id === 'string') : [];
        const collapsed = parsed.collapsed && typeof parsed.collapsed === 'object' ? parsed.collapsed : {};
        return {
            order,
            collapsed
        };
    } catch (_) {
        return { order: DASHBOARD_SECTION_ORDER.slice(), collapsed: {} };
    }
}

function saveDashboardLayout(projectId) {
    const container = document.getElementById('dashboard-sections');
    if (!container) return;

    const order = Array.from(container.querySelectorAll('.dashboard-section'))
        .map(section => section.dataset.sectionId)
        .filter(Boolean);

    const collapsed = {};
    container.querySelectorAll('.dashboard-section').forEach(section => {
        const id = section.dataset.sectionId;
        if (!id) return;
        collapsed[id] = section.classList.contains('section-collapsed');
    });

    try {
        localStorage.setItem(
            dashboardLayoutStorageKey(projectId),
            JSON.stringify({ order, collapsed })
        );
    } catch (_) {
        // Ignore storage write errors and keep runtime state only.
    }
}

function setSectionCollapsed(section, collapsed) {
    section.classList.toggle('section-collapsed', collapsed);
    const toggle = section.querySelector('[data-section-toggle]');
    if (toggle) {
        toggle.textContent = collapsed ? '►' : '▼';
        toggle.setAttribute('aria-expanded', collapsed ? 'false' : 'true');
    }
}

function applyDashboardLayout(projectId) {
    const container = document.getElementById('dashboard-sections');
    if (!container) return;

    const layout = loadDashboardLayout(projectId);
    const sectionByID = {};
    Array.from(container.querySelectorAll('.dashboard-section')).forEach(section => {
        const id = section.dataset.sectionId;
        if (id) {
            sectionByID[id] = section;
        }
    });

    const applied = new Set();
    const requestedOrder = layout.order && layout.order.length > 0 ? layout.order : DASHBOARD_SECTION_ORDER;
    requestedOrder.forEach(id => {
        const section = sectionByID[id];
        if (!section) return;
        container.appendChild(section);
        applied.add(id);
    });

    Array.from(container.querySelectorAll('.dashboard-section')).forEach(section => {
        const id = section.dataset.sectionId;
        if (!id || applied.has(id)) return;
        container.appendChild(section);
    });

    Array.from(container.querySelectorAll('.dashboard-section')).forEach(section => {
        const id = section.dataset.sectionId;
        const collapsed = !!layout.collapsed[id];
        setSectionCollapsed(section, collapsed);
    });
}

function initDashboardSections(projectId) {
    const container = document.getElementById('dashboard-sections');
    if (!container) return;

    applyDashboardLayout(projectId);
    updateToggleAllSectionsButtonLabel();

    let dragHandleSectionID = '';
    let draggingSectionID = '';

    const clearDropHighlights = () => {
        container.querySelectorAll('.dashboard-section.drag-over-target').forEach(section => {
            section.classList.remove('drag-over-target');
        });
    };

    container.querySelectorAll('.dashboard-section').forEach(section => {
        const sectionID = section.dataset.sectionId;
        if (!sectionID) return;

        const handle = section.querySelector('.section-drag-handle');
        if (handle) {
            handle.addEventListener('mousedown', () => {
                dragHandleSectionID = sectionID;
            });
            handle.addEventListener('mouseup', () => {
                dragHandleSectionID = '';
            });
            handle.addEventListener('mouseleave', () => {
                dragHandleSectionID = '';
            });
        }

        section.addEventListener('dragstart', (event) => {
            if (dragHandleSectionID !== sectionID) {
                event.preventDefault();
                return;
            }
            draggingSectionID = sectionID;
            section.classList.add('dragging');
            if (event.dataTransfer) {
                event.dataTransfer.effectAllowed = 'move';
                event.dataTransfer.setData('text/plain', sectionID);
            }
        });

        section.addEventListener('dragend', () => {
            section.classList.remove('dragging');
            clearDropHighlights();
            dragHandleSectionID = '';
            draggingSectionID = '';
            saveDashboardLayout(projectId);
        });

        section.addEventListener('dragover', (event) => {
            if (!draggingSectionID || draggingSectionID === sectionID) {
                return;
            }
            event.preventDefault();
            section.classList.add('drag-over-target');
        });

        section.addEventListener('dragleave', () => {
            section.classList.remove('drag-over-target');
        });

        section.addEventListener('drop', (event) => {
            if (!draggingSectionID || draggingSectionID === sectionID) {
                return;
            }
            event.preventDefault();
            clearDropHighlights();

            const sections = Array.from(container.querySelectorAll('.dashboard-section'));
            const draggedSection = sections.find(item => item.dataset.sectionId === draggingSectionID);
            if (!draggedSection) return;

            const targetRect = section.getBoundingClientRect();
            const insertAfter = event.clientY > targetRect.top + (targetRect.height / 2);
            container.insertBefore(draggedSection, insertAfter ? section.nextSibling : section);
            saveDashboardLayout(projectId);
        });

        const collapseBtn = section.querySelector('[data-section-toggle]');
        if (collapseBtn) {
            collapseBtn.addEventListener('click', () => {
                const collapsed = !section.classList.contains('section-collapsed');
                setSectionCollapsed(section, collapsed);
                saveDashboardLayout(projectId);
                updateToggleAllSectionsButtonLabel();
            });
        }
    });
}

function setAllSectionsCollapsed(projectId, collapsed) {
    const container = document.getElementById('dashboard-sections');
    if (!container) return;

    container.querySelectorAll('.dashboard-section').forEach(section => {
        setSectionCollapsed(section, collapsed);
    });
    saveDashboardLayout(projectId);
    updateToggleAllSectionsButtonLabel();
}

function areAllSectionsCollapsed() {
    const container = document.getElementById('dashboard-sections');
    if (!container) return false;
    const sections = Array.from(container.querySelectorAll('.dashboard-section'));
    if (sections.length === 0) return false;
    return sections.every(section => section.classList.contains('section-collapsed'));
}

function updateToggleAllSectionsButtonLabel() {
    const btn = document.getElementById('toggle-all-sections-btn');
    if (!btn) return;
    btn.textContent = areAllSectionsCollapsed() ? 'Expand All' : 'Collapse All';
}

function initBaselineExpectedUnseenControls() {
    const searchInput = document.getElementById('baseline-expected-unseen-search');
    const prevBtn = document.getElementById('baseline-expected-unseen-prev');
    const nextBtn = document.getElementById('baseline-expected-unseen-next');

    if (searchInput) {
        searchInput.addEventListener('input', () => {
            baselineExpectedUnseenState.query = searchInput.value.trim();
            baselineExpectedUnseenState.page = 1;
            renderBaselineExpectedUnseenList();
        });
    }

    if (prevBtn) {
        prevBtn.addEventListener('click', () => {
            if (baselineExpectedUnseenState.page <= 1) return;
            baselineExpectedUnseenState.page -= 1;
            renderBaselineExpectedUnseenList();
        });
    }

    if (nextBtn) {
        nextBtn.addEventListener('click', () => {
            const filtered = getBaselineExpectedUnseenFilteredItems();
            const totalPages = Math.max(1, Math.ceil(filtered.length / baselineExpectedUnseenState.pageSize));
            if (baselineExpectedUnseenState.page >= totalPages) return;
            baselineExpectedUnseenState.page += 1;
            renderBaselineExpectedUnseenList();
        });
    }
}

function getBaselineExpectedUnseenFilteredItems() {
    const query = baselineExpectedUnseenState.query.toLowerCase();
    if (!query) {
        return baselineExpectedUnseenState.allItems.slice();
    }
    return baselineExpectedUnseenState.allItems.filter(item => String(item).toLowerCase().includes(query));
}

function renderBaselineExpectedUnseenList() {
    const list = document.getElementById('baseline-list-expected-unseen');
    const count = document.getElementById('baseline-expected-unseen-count');
    const prevBtn = document.getElementById('baseline-expected-unseen-prev');
    const nextBtn = document.getElementById('baseline-expected-unseen-next');
    if (!list || !count || !prevBtn || !nextBtn) return;

    const filtered = getBaselineExpectedUnseenFilteredItems();
    const pageSize = baselineExpectedUnseenState.pageSize;
    const total = filtered.length;
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (baselineExpectedUnseenState.page > totalPages) {
        baselineExpectedUnseenState.page = totalPages;
    }

    const startIndex = total === 0 ? 0 : (baselineExpectedUnseenState.page - 1) * pageSize;
    const endIndex = Math.min(startIndex + pageSize, total);
    const items = filtered.slice(startIndex, endIndex);

    list.innerHTML = '';
    if (items.length === 0) {
        const empty = document.createElement('li');
        empty.className = 'text-muted';
        empty.textContent = baselineExpectedUnseenState.query
            ? 'No hosts match current filter.'
            : 'None';
        list.appendChild(empty);
    } else {
        items.forEach(item => {
            const li = document.createElement('li');
            li.textContent = String(item);
            list.appendChild(li);
        });
    }

    const from = total === 0 ? 0 : startIndex + 1;
    const to = total === 0 ? 0 : endIndex;
    count.textContent = `Showing ${from}-${to} of ${total}`;

    prevBtn.disabled = baselineExpectedUnseenState.page <= 1;
    nextBtn.disabled = baselineExpectedUnseenState.page >= totalPages;
}

document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    initDashboardSections(projectId);
    initBaselineExpectedUnseenControls();

    const toggleAllSectionsBtn = document.getElementById('toggle-all-sections-btn');
    if (toggleAllSectionsBtn) {
        toggleAllSectionsBtn.addEventListener('click', () => {
            const shouldExpandAll = areAllSectionsCollapsed();
            setAllSectionsCollapsed(projectId, !shouldExpandAll);
        });
    }

    try {
        const [project, stats] = await Promise.all([
            api(`/projects/${projectId}`),
            api(`/projects/${projectId}/stats`)
        ]);

        document.title = `NmapTracker - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('project-title').textContent = project.Name;

        document.getElementById('view-hosts-btn').href = `hosts.html?id=${projectId}`;
        document.getElementById('view-all-scans-btn').href = `scan_results.html?id=${projectId}`;
        document.getElementById('view-coverage-matrix-btn').href = `coverage_matrix.html?id=${projectId}`;
        document.getElementById('view-import-delta-btn').href = `import_delta.html?id=${projectId}`;
        document.getElementById('view-service-queues-btn').href = `service_queues.html?id=${projectId}`;
        document.getElementById('link-total-hosts').href = `hosts.html?id=${projectId}`;
        document.getElementById('link-in-scope').href = `hosts.html?id=${projectId}&in_scope=true`;
        document.getElementById('link-out-scope').href = `hosts.html?id=${projectId}&in_scope=false`;
        document.getElementById('queue-smb-btn').href = `service_queues.html?id=${projectId}&campaign=smb`;
        document.getElementById('queue-ldap-btn').href = `service_queues.html?id=${projectId}&campaign=ldap`;
        document.getElementById('queue-rdp-btn').href = `service_queues.html?id=${projectId}&campaign=rdp`;
        document.getElementById('queue-http-btn').href = `service_queues.html?id=${projectId}&campaign=http`;

        document.getElementById('link-wf-scanned').href = `scan_results.html?id=${projectId}&status=scanned`;
        document.getElementById('link-wf-flagged').href = `scan_results.html?id=${projectId}&status=flagged`;
        document.getElementById('link-wf-in-progress').href = `scan_results.html?id=${projectId}&status=in_progress`;
        document.getElementById('link-wf-done').href = `scan_results.html?id=${projectId}&status=done`;

        document.getElementById('export-json-btn').href = `/api/projects/${projectId}/export?format=json`;
        document.getElementById('export-csv-btn').href = `/api/projects/${projectId}/export?format=csv`;
        document.getElementById('export-text-btn').href = `/api/projects/${projectId}/export?format=text`;

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.dropdown')) {
                const menu = document.getElementById('project-tools-menu');
                if (menu) {
                    menu.style.display = 'none';
                }
            }
        });

        renderStats(stats);
        loadScopeRules();
        setupImport();
        loadBaseline();

        const refreshImportIntents = document.getElementById('refresh-import-intents-btn');
        if (refreshImportIntents) {
            refreshImportIntents.addEventListener('click', () => loadImportIntents());
        }
        const saveAllImportIntentsBtn = document.getElementById('save-all-import-intents-btn');
        if (saveAllImportIntentsBtn) {
            saveAllImportIntentsBtn.addEventListener('click', () => saveAllImportIntents());
        }
        loadImportIntents();

    } catch (err) {
        console.error(err);
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});

function renderStats(stats) {
    document.getElementById('stat-total').textContent = stats.TotalHosts;
    document.getElementById('stat-in-scope').textContent = stats.InScopeHosts;
    document.getElementById('stat-out-scope').textContent = stats.OutScopeHosts;

    document.getElementById('stat-scanned').textContent = stats.WorkStatus.Scanned;
    document.getElementById('stat-flagged').textContent = stats.WorkStatus.Flagged;
    document.getElementById('stat-in-progress').textContent = stats.WorkStatus.InProgress;
    document.getElementById('stat-done').textContent = stats.WorkStatus.Done;

    if (stats.InScopeHosts > 0) {
        const totalPorts = stats.WorkStatus.Scanned + stats.WorkStatus.Flagged + stats.WorkStatus.InProgress + stats.WorkStatus.Done;
        let pct = 0;
        if (totalPorts > 0) {
            pct = Math.round((stats.WorkStatus.Done / totalPorts) * 100);
        }
        const pctStr = `${pct}%`;
        document.getElementById('progress-percent').textContent = pctStr;
        document.getElementById('progress-fill').style.width = pctStr;
    } else {
        document.getElementById('progress-percent').textContent = 'N/A';
        document.getElementById('progress-fill').style.width = '0%';
    }
}

async function loadDashboardStats() {
    const projectId = getProjectId();
    try {
        const stats = await api(`/projects/${projectId}/stats`);
        renderStats(stats);
    } catch (err) {
        console.error('Failed to refresh stats', err);
    }
}

function renderGapHostList(listId, items, formatter) {
    const list = document.getElementById(listId);
    if (!list) return;

    list.innerHTML = '';
    if (!items || items.length === 0) {
        const empty = document.createElement('li');
        empty.className = 'text-muted';
        empty.textContent = 'None';
        list.appendChild(empty);
        return;
    }

    items.forEach(item => {
        const li = document.createElement('li');
        li.textContent = formatter(item);
        list.appendChild(li);
    });
}

async function loadImportIntents() {
    const projectId = getProjectId();
    if (!projectId) return;

    try {
        const resp = await api(`/projects/${projectId}/imports`);
        importIntentsCache = (resp && resp.items) ? resp.items : [];
        renderImportIntents(importIntentsCache);
    } catch (err) {
        console.error('Failed to load import intents', err);
        showToast(`Failed to load import intents: ${err.message}`, 'error');
    }
}

function renderImportIntents(items) {
    const tbody = document.getElementById('import-intents-body');
    if (!tbody) return;

    tbody.innerHTML = '';
    if (!items || items.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = 4;
        td.className = 'text-muted';
        td.style.textAlign = 'center';
        td.textContent = 'No imports yet.';
        tr.appendChild(td);
        tbody.appendChild(tr);
        return;
    }

    items.forEach(item => {
        const tr = document.createElement('tr');
        tr.dataset.importId = String(item.id);

        const currentIntents = new Set((item.intents || []).map(intent => String(intent.intent || '').toLowerCase()));
        const importTime = item.import_time ? new Date(item.import_time).toLocaleString() : '-';

        const idTd = document.createElement('td');
        idTd.textContent = String(item.id);

        const timeTd = document.createElement('td');
        timeTd.textContent = importTime;

        const fileTd = document.createElement('td');
        fileTd.textContent = item.filename || '-';

        const intentsTd = document.createElement('td');
        intentsTd.style.whiteSpace = 'normal';

        IMPORT_INTENT_OPTIONS.forEach(opt => {
            const wrapper = document.createElement('label');
            wrapper.style.display = 'inline-flex';
            wrapper.style.alignItems = 'center';
            wrapper.style.gap = '6px';
            wrapper.style.marginRight = '12px';
            wrapper.style.marginBottom = '6px';

            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.checked = currentIntents.has(opt.id);
            checkbox.dataset.intent = opt.id;
            checkbox.dataset.importId = String(item.id);

            const text = document.createElement('span');
            text.textContent = opt.label;

            wrapper.appendChild(checkbox);
            wrapper.appendChild(text);
            intentsTd.appendChild(wrapper);
        });

        tr.appendChild(idTd);
        tr.appendChild(timeTd);
        tr.appendChild(fileTd);
        tr.appendChild(intentsTd);
        tbody.appendChild(tr);
    });
}

function getSelectedIntentsForRow(row) {
    return Array.from(row.querySelectorAll('input[type="checkbox"][data-intent]:checked'))
        .map(checkbox => ({
            intent: checkbox.dataset.intent,
            source: 'manual',
            confidence: 1
        }));
}

async function saveImportIntentsForRow(projectId, row, importId) {
    const selected = getSelectedIntentsForRow(row);
    await api(`/projects/${projectId}/imports/${importId}/intents`, {
        method: 'PUT',
        body: JSON.stringify({ intents: selected })
    });
}

async function saveAllImportIntents() {
    const projectId = getProjectId();
    if (!projectId) return;

    const rows = Array.from(document.querySelectorAll('#import-intents-body tr[data-import-id]'));
    if (rows.length === 0) {
        showToast('No imports to save.', 'error');
        return;
    }

    const saveAllImportIntentsBtn = document.getElementById('save-all-import-intents-btn');
    const originalLabel = saveAllImportIntentsBtn ? saveAllImportIntentsBtn.textContent : '';
    if (saveAllImportIntentsBtn) {
        saveAllImportIntentsBtn.disabled = true;
        saveAllImportIntentsBtn.textContent = 'Saving...';
    }

    try {
        const results = [];
        for (const row of rows) {
            const importId = Number(row.dataset.importId);
            if (!Number.isInteger(importId)) {
                results.push({ ok: false, importId: row.dataset.importId || '?' });
                continue;
            }
            try {
                await saveImportIntentsForRow(projectId, row, importId);
                results.push({ ok: true, importId });
            } catch (_) {
                results.push({ ok: false, importId });
            }
        }

        const successCount = results.filter(item => item.ok).length;
        const failed = results.filter(item => !item.ok);

        if (successCount > 0) {
            showToast(`Saved intents for ${successCount} import${successCount === 1 ? '' : 's'}`, 'success');
            await loadImportIntents();
        }
        if (failed.length > 0) {
            const failedSummary = failed
                .slice(0, 5)
                .map(item => `#${item.importId}`)
                .join(', ');
            const suffix = failed.length > 5 ? ` (+${failed.length - 5} more)` : '';
            showToast(`Failed to save intents for ${failedSummary}${suffix}`, 'error');
        }
    } finally {
        if (saveAllImportIntentsBtn) {
            saveAllImportIntentsBtn.disabled = false;
            saveAllImportIntentsBtn.textContent = originalLabel || 'Save All';
        }
    }
}

async function loadBaseline() {
    const projectId = getProjectId();
    if (!projectId) return;

    try {
        const resp = await api(`/projects/${projectId}/baseline`);
        renderBaselineRules(resp && resp.items ? resp.items : []);
        await evaluateBaseline({ silentError: true });
    } catch (err) {
        console.error('Failed to load baseline', err);
        showToast(`Failed to load baseline: ${err.message}`, 'error');
    }
}

function renderBaselineRules(items) {
    const list = document.getElementById('baseline-rules-list');
    const empty = document.getElementById('baseline-empty');
    if (!list || !empty) return;

    list.innerHTML = '';
    if (!items || items.length === 0) {
        empty.style.display = 'block';
        return;
    }

    empty.style.display = 'none';
    items.forEach(item => {
        const row = document.createElement('li');
        row.className = 'scope-rule-item';

        const textWrap = document.createElement('span');
        const defText = document.createElement('span');
        defText.textContent = item.definition;
        const typeText = document.createElement('span');
        typeText.className = 'rule-type';
        typeText.textContent = item.type;
        textWrap.appendChild(defText);
        textWrap.appendChild(typeText);

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'delete-btn';
        deleteBtn.title = 'Remove';
        deleteBtn.textContent = '×';
        deleteBtn.addEventListener('click', () => deleteBaseline(item.id));

        row.appendChild(textWrap);
        row.appendChild(deleteBtn);
        list.appendChild(row);
    });
}

async function addBaseline() {
    const input = document.getElementById('baseline-input');
    if (!input) return;

    const definitions = input.value
        .split('\n')
        .map(line => line.trim())
        .filter(line => line.length > 0);
    if (definitions.length === 0) {
        showToast('Enter at least one IPv4 address or CIDR', 'error');
        return;
    }

    const projectId = getProjectId();
    try {
        const resp = await api(`/projects/${projectId}/baseline`, {
            method: 'POST',
            body: JSON.stringify({ definitions })
        });
        showToast(`Added ${resp.added || 0} baseline definition(s)`, 'success');
        input.value = '';
        await loadBaseline();
    } catch (err) {
        showToast(`Failed to add baseline: ${err.message}`, 'error');
    }
}

async function deleteBaseline(baselineId) {
    const projectId = getProjectId();
    try {
        await api(`/projects/${projectId}/baseline/${baselineId}`, { method: 'DELETE' });
        showToast('Baseline definition removed', 'success');
        await loadBaseline();
    } catch (err) {
        showToast(`Failed to remove baseline: ${err.message}`, 'error');
    }
}

async function evaluateBaseline(options = {}) {
    const projectId = getProjectId();
    if (!projectId) return;

    try {
        const result = await api(`/projects/${projectId}/baseline/evaluate`);
        renderBaselineEvaluation(result);
    } catch (err) {
        if (!options.silentError) {
            showToast(`Baseline evaluation failed: ${err.message}`, 'error');
        }
    }
}

function renderBaselineEvaluation(result) {
    const summary = result && result.summary ? result.summary : {};
    const lists = result && result.lists ? result.lists : {};

    const generatedAt = document.getElementById('baseline-generated-at');
    if (generatedAt) {
        generatedAt.textContent = result && result.generated_at ? `Generated ${result.generated_at}` : 'Not evaluated yet';
    }

    const expectedTotal = document.getElementById('baseline-summary-expected-total');
    if (expectedTotal) expectedTotal.textContent = summary.expected_total || 0;
    const observedTotal = document.getElementById('baseline-summary-observed-total');
    if (observedTotal) observedTotal.textContent = summary.observed_total || 0;
    const expectedUnseen = document.getElementById('baseline-summary-expected-unseen');
    if (expectedUnseen) expectedUnseen.textContent = summary.expected_but_unseen || 0;
    const seenOutScope = document.getElementById('baseline-summary-seen-out-scope');
    if (seenOutScope) seenOutScope.textContent = summary.seen_but_out_of_scope || 0;

    baselineExpectedUnseenState.allItems = (lists.expected_but_unseen || []).map(item => String(item));
    baselineExpectedUnseenState.page = 1;
    renderBaselineExpectedUnseenList();

    renderGapHostList(
        'baseline-list-seen-out-scope',
        lists.seen_but_out_of_scope || [],
        item => `${item.ip_address}${item.hostname ? ` (${item.hostname})` : ''}${item.in_scope ? ' [marked in-scope]' : ' [marked out-of-scope]'}`
    );
}

async function loadScopeRules() {
    const projectId = getProjectId();
    const rules = await api(`/projects/${projectId}/scope`);
    renderScopeRules(rules);
}

function renderScopeRules(rules) {
    const list = document.getElementById('scope-rules-list');
    const empty = document.getElementById('scope-empty');

    if (!rules || rules.length === 0) {
        list.innerHTML = '';
        empty.style.display = 'block';
        return;
    }

    empty.style.display = 'none';
    list.innerHTML = '';
    rules.forEach(rule => {
        const item = document.createElement('li');
        item.className = 'scope-rule-item';

        const textWrap = document.createElement('span');
        const defText = document.createElement('span');
        defText.textContent = rule.Definition;
        const typeText = document.createElement('span');
        typeText.className = 'rule-type';
        typeText.textContent = rule.Type;
        textWrap.appendChild(defText);
        textWrap.appendChild(typeText);

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'delete-btn';
        deleteBtn.title = 'Remove';
        deleteBtn.textContent = '×';
        deleteBtn.addEventListener('click', () => deleteScopeRule(rule.ID));

        item.appendChild(textWrap);
        item.appendChild(deleteBtn);
        list.appendChild(item);
    });
}

async function addScopeRules() {
    const input = document.getElementById('scope-input');
    const lines = input.value.split('\n')
        .map(l => l.trim())
        .filter(l => l.length > 0);

    if (lines.length === 0) {
        showToast('Enter at least one IP or CIDR', 'error');
        return;
    }

    const projectId = getProjectId();

    try {
        const result = await api(`/projects/${projectId}/scope`, {
            method: 'POST',
            body: JSON.stringify({ definitions: lines })
        });

        showToast(`Added ${result.added} scope rule(s)`, 'success');
        input.value = '';
        renderScopeRules(result.rules);
    } catch (err) {
        showToast(`Failed to add scope rules: ${err.message}`, 'error');
    }
}

async function deleteScopeRule(ruleId) {
    const projectId = getProjectId();

    try {
        await api(`/projects/${projectId}/scope/${ruleId}`, {
            method: 'DELETE'
        });
        showToast('Scope rule removed', 'success');
        loadScopeRules();
    } catch (err) {
        showToast(`Failed to remove rule: ${err.message}`, 'error');
    }
}

async function reEvaluateScope() {
    const projectId = getProjectId();

    try {
        const result = await api(`/projects/${projectId}/scope/evaluate`, {
            method: 'POST'
        });
        showToast(`Updated ${result.updated} hosts (${result.in_scope} in scope, ${result.out_of_scope} out of scope)`, 'success');
        loadDashboardStats();
    } catch (err) {
        showToast(`Failed to re-evaluate scope: ${err.message}`, 'error');
    }
}

let selectedFiles = [];

function setupImport() {
    const dropzone = document.getElementById('import-dropzone');
    const fileInput = document.getElementById('import-file');

    dropzone.addEventListener('dragover', (e) => {
        e.preventDefault();
        dropzone.classList.add('drag-over');
    });

    dropzone.addEventListener('dragleave', () => {
        dropzone.classList.remove('drag-over');
    });

    dropzone.addEventListener('drop', (e) => {
        e.preventDefault();
        dropzone.classList.remove('drag-over');

        const files = e.dataTransfer.files;
        if (files.length > 0) {
            handleFiles(files);
        }
    });

    fileInput.addEventListener('change', (e) => {
        if (e.target.files.length > 0) {
            handleFiles(e.target.files);
        }
    });
}

function handleFiles(files) {
    const validFiles = [];
    for (let i = 0; i < files.length; i++) {
        if (files[i].name.endsWith('.xml')) {
            validFiles.push(files[i]);
        }
    }

    if (validFiles.length === 0) {
        showToast('Please select at least one XML file', 'error');
        return;
    }

    selectedFiles = validFiles;

    const fileCount = selectedFiles.length;
    const countText = fileCount === 1 ? selectedFiles[0].name : `${fileCount} files selected`;

    document.getElementById('import-filename').textContent = countText;
    document.getElementById('import-dropzone').style.display = 'none';
    document.getElementById('import-status').style.display = 'flex';
}

function clearImport() {
    selectedFiles = [];
    document.getElementById('import-file').value = '';
    document.getElementById('import-dropzone').style.display = 'block';
    document.getElementById('import-status').style.display = 'none';
}

async function uploadFile() {
    if (selectedFiles.length === 0) return;

    const projectId = getProjectId();
    document.getElementById('import-status').style.display = 'none';
    document.getElementById('import-progress').style.display = 'block';

    let totalHosts = 0;
    let totalPorts = 0;
    const errors = [];

    for (let i = 0; i < selectedFiles.length; i++) {
        const file = selectedFiles[i];

        const statusText = document.getElementById('import-progress').querySelector('p');
        if (statusText) {
            statusText.textContent = `Importing ${i + 1} of ${selectedFiles.length}...`;
        }

        const formData = new FormData();
        formData.append('file', file);

        try {
            const response = await fetch(`/api/projects/${projectId}/import`, {
                method: 'POST',
                body: formData
            });

            if (!response.ok) {
                const text = await response.text();
                throw new Error(text || 'Import failed');
            }

            const result = await response.json();
            totalHosts += result.hosts_imported;
            totalPorts += result.ports_imported;

        } catch (err) {
            console.error(`Failed to import ${file.name}:`, err);
            errors.push(`${file.name}: ${err.message}`);
        }
    }

    document.getElementById('import-progress').style.display = 'none';

    if (errors.length > 0) {
        if (totalHosts > 0) {
            showToast(`Imported ${totalHosts} hosts, ${totalPorts} ports. Failures: ${errors.join(', ')}`, 'warning');
        } else {
            showToast(`Import failed. Errors: ${errors.join(', ')}`, 'error');
            document.getElementById('import-status').style.display = 'flex';
            return;
        }
    } else {
        showToast(`Successfully imported ${totalHosts} hosts, ${totalPorts} ports from ${selectedFiles.length} files.`, 'success');
    }

    if (errors.length === 0 || totalHosts > 0) {
        clearImport();
        document.getElementById('import-dropzone').style.display = 'block';
        loadDashboardStats();
        loadImportIntents();
    }
}

function toggleEditName() {
    const titleContainer = document.getElementById('page-title-container');
    const form = document.getElementById('rename-form');

    if (form.style.display === 'none') {
        titleContainer.style.display = 'none';
        form.style.display = 'flex';
        const currentName = document.getElementById('project-title').textContent;
        document.getElementById('rename-input').value = currentName;
        document.getElementById('rename-input').focus();
    } else {
        cancelRename();
    }
}

function cancelRename() {
    document.getElementById('page-title-container').style.display = 'block';
    document.getElementById('rename-form').style.display = 'none';
}

async function saveProjectName() {
    const projectId = getProjectId();
    const newName = document.getElementById('rename-input').value.trim();
    if (!newName) return;

    try {
        await api(`/projects/${projectId}`, {
            method: 'PUT',
            body: JSON.stringify({ name: newName })
        });
        document.getElementById('project-title').textContent = newName;
        document.getElementById('nav-project-name').textContent = newName;
        document.title = `NmapTracker - ${newName}`;
        cancelRename();
        showToast('Project renamed', 'success');
    } catch (err) {
        showToast(err.message, 'error');
    }
}

function toggleProjectToolsMenu() {
    const menu = document.getElementById('project-tools-menu');
    if (!menu) return;
    menu.style.display = menu.style.display === 'block' ? 'none' : 'block';
}

window.toggleProjectToolsMenu = toggleProjectToolsMenu;
window.toggleEditName = toggleEditName;
window.saveProjectName = saveProjectName;
window.cancelRename = cancelRename;
window.addBaseline = addBaseline;
window.evaluateBaseline = evaluateBaseline;
window.addScopeRules = addScopeRules;
window.reEvaluateScope = reEvaluateScope;
window.uploadFile = uploadFile;
window.clearImport = clearImport;
