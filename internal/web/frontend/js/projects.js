document.addEventListener('DOMContentLoaded', () => {
    loadProjects();

    const createBtn = document.getElementById('create-btn');
    const createForm = document.getElementById('create-form');
    const cancelBtn = document.getElementById('cancel-project-btn');
    const saveBtn = document.getElementById('save-project-btn');
    const nameInput = document.getElementById('project-name');

    createBtn.addEventListener('click', () => {
        createForm.style.display = 'block';
        createBtn.style.display = 'none';
        nameInput.focus();
    });

    cancelBtn.addEventListener('click', () => {
        createForm.style.display = 'none';
        createBtn.style.display = 'block';
        nameInput.value = '';
    });

    saveBtn.addEventListener('click', async () => {
        const name = nameInput.value.trim();
        if (!name) return;

        try {
            await api('/projects', {
                method: 'POST',
                body: JSON.stringify({ name })
            });
            nameInput.value = '';
            createForm.style.display = 'none';
            createBtn.style.display = 'block';
            loadProjects();
        } catch (err) {
            showError(err.message);
        }
    });
});

async function loadProjects() {
    try {
        const projects = await api('/projects');
        const tbody = document.getElementById('projects-list');
        tbody.innerHTML = '';

        projects.forEach(p => {
            const tr = document.createElement('tr');

            const tdId = document.createElement('td');
            tdId.textContent = p.ID;

            const tdName = document.createElement('td');
            const link = document.createElement('a');
            link.href = `project.html?id=${p.ID}`;
            link.textContent = p.Name;
            tdName.appendChild(link);

            const tdDate = document.createElement('td');
            tdDate.textContent = new Date(p.CreatedAt).toLocaleString();

            const tdActions = document.createElement('td');
            const delBtn = document.createElement('button');
            delBtn.textContent = 'Delete';
            delBtn.className = 'danger';
            delBtn.onclick = () => deleteProject(p.ID, p.Name);
            tdActions.appendChild(delBtn);

            tr.appendChild(tdId);
            tr.appendChild(tdName);
            tr.appendChild(tdDate);
            tr.appendChild(tdActions);
            tbody.appendChild(tr);
        });
    } catch (err) {
        showError(err.message);
    }
}

async function deleteProject(id, name) {
    if (!confirm(`Are you sure you want to delete project "${name}"?`)) return;

    try {
        await api(`/projects/${id}`, { method: 'DELETE' });
        loadProjects();
    } catch (err) {
        showError(err.message);
    }
}

function showError(msg) {
    const el = document.getElementById('error-msg');
    el.textContent = msg;
    el.style.display = 'block';
    setTimeout(() => el.style.display = 'none', 5000);
}
