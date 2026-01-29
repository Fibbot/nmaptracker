// Base API URL
const API = '/api';

// Fetch wrapper with error handling
async function api(path, options = {}) {
    const res = await fetch(API + path, {
        headers: { 'Content-Type': 'application/json', ...options.headers },
        ...options
    });
    if (!res.ok) {
        const text = await res.text();
        throw new Error(text || res.statusText);
    }
    if (res.status === 204) return null;
    try {
        return await res.json();
    } catch (e) {
        return null; // Handle empty body if not 204 but no content
    }
}

// Get URL params
function getParam(name) {
    return new URLSearchParams(window.location.search).get(name);
}

// Get path segment helpers
function getProjectId() {
    return getParam('id');
}

function getHostId() {
    return getParam('hostId');
}

// DOM Helper
function el(tag, className, text) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (text) element.textContent = text;
    return element;
}
