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

        // Use standard links for exports that trigger downloads
        const exportDiv = document.getElementById('export-links');
        exportDiv.innerHTML = `
            <a href="/api/projects/${projectId}/export?format=json" target="_blank" class="button secondary">Export JSON</a>
            <a href="/api/projects/${projectId}/export?format=csv" target="_blank" class="button secondary">Export CSV</a>
        `;
        // Hack: The button class needs to be added to the a tag styles if I want them to look like buttons
        // For now, I'll just add inline styles or rely on the global button style if I used <button>
        // But since they are <a> tags, let's just make them look like links or add a class in CSS later.
        // Actually, style.css has button styles but not a.button styles. Let's just leave them as text links styled slightly.
        const links = exportDiv.querySelectorAll('a');
        links.forEach(l => {
            l.style.marginLeft = '10px';
            l.style.textDecoration = 'none';
            l.style.padding = '8px 16px';
            l.style.background = '#6c757d';
            l.style.color = 'white';
            l.style.borderRadius = '4px';
        });


        // Stats
        document.getElementById('stat-total').textContent = stats.TotalHosts;
        document.getElementById('stat-in-scope').textContent = stats.InScopeHosts;
        document.getElementById('stat-out-scope').textContent = stats.OutScopeHosts;

        document.getElementById('stat-scanned').textContent = stats.WorkStatus.Scanned;
        document.getElementById('stat-flagged').textContent = stats.WorkStatus.Flagged;
        document.getElementById('stat-in-progress').textContent = stats.WorkStatus.InProgress;
        document.getElementById('stat-done').textContent = stats.WorkStatus.Done;
        document.getElementById('stat-parking').textContent = stats.WorkStatus.ParkingLot;

        // Progress
        if (stats.InScopeHosts > 0) {
            const pct = Math.round((stats.WorkStatus.Done / stats.InScopeHosts) * 100);
            document.getElementById('progress-percent').textContent = `${pct}%`;
        } else {
            document.getElementById('progress-percent').textContent = 'N/A';
        }

    } catch (err) {
        document.getElementById('error-msg').textContent = err.message;
        document.getElementById('error-msg').style.display = 'block';
    }
});
