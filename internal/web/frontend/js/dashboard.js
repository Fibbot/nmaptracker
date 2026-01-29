document.addEventListener('DOMContentLoaded', async () => {
    const projectId = getProjectId();
    if (!projectId) {
        window.location.href = 'index.html';
        return;
    }

    try {
        const [project, stats] = await Promise.all([
            api(`/projects/${projectId}`),
            api(`/projects/${projectId}/stats`)
        ]);

        // Set Headers
        document.title = `NmapTracker - ${project.Name}`;
        document.getElementById('nav-project-name').textContent = project.Name;
        document.getElementById('nav-project-name').href = `project.html?id=${projectId}`;
        document.getElementById('project-title').textContent = project.Name;

        // Links
        document.getElementById('view-hosts-btn').href = `hosts.html?id=${projectId}`;
        document.getElementById('link-total-hosts').href = `hosts.html?id=${projectId}`;
        document.getElementById('link-in-scope').href = `hosts.html?id=${projectId}&in_scope=true`;
        document.getElementById('link-out-scope').href = `hosts.html?id=${projectId}&in_scope=false`;

        // Export Links
        const exportDiv = document.getElementById('export-links');
        exportDiv.innerHTML = `
            <a href="/api/projects/${projectId}/export?format=json" target="_blank" class="btn btn-secondary">Export JSON</a>
            <a href="/api/projects/${projectId}/export?format=csv" target="_blank" class="btn btn-secondary">Export CSV</a>
        `;

        // Stats - Hosts
        document.getElementById('stat-total').textContent = stats.TotalHosts;
        document.getElementById('stat-in-scope').textContent = stats.InScopeHosts;
        document.getElementById('stat-out-scope').textContent = stats.OutScopeHosts;

        // Stats - Workflow
        document.getElementById('stat-scanned').textContent = stats.WorkStatus.Scanned;
        document.getElementById('stat-flagged').textContent = stats.WorkStatus.Flagged;
        document.getElementById('stat-in-progress').textContent = stats.WorkStatus.InProgress;
        document.getElementById('stat-done').textContent = stats.WorkStatus.Done;
        document.getElementById('stat-parking').textContent = stats.WorkStatus.ParkingLot;

        // Progress
        if (stats.InScopeHosts > 0) {
            const pct = Math.round((stats.WorkStatus.Done / stats.InScopeHosts) * 100);
            const pctStr = `${pct}%`;
            document.getElementById('progress-percent').textContent = pctStr;
            document.getElementById('progress-fill').style.width = pctStr;
        } else {
            document.getElementById('progress-percent').textContent = 'N/A';
            document.getElementById('progress-fill').style.width = '0%';
        }

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});
