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
        return null;
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

// --- Toast Notifications ---
function showToast(message, type = 'info', duration = 3000) {
    const container = document.getElementById('toast-container')
        || createToastContainer();

    const toast = document.createElement('div');
    toast.className = `toast toast-${type}`;
    toast.textContent = message;

    container.appendChild(toast);

    setTimeout(() => {
        toast.classList.add('removing');
        setTimeout(() => toast.remove(), 200);
    }, duration);
}

function createToastContainer() {
    const container = document.createElement('div');
    container.id = 'toast-container';
    container.className = 'toast-container';
    document.body.appendChild(container);
    return container;
}

// --- Client-side Sorting ---
function makeSortable(table) {
    const headers = table.querySelectorAll('th.sortable');
    const tbody = table.querySelector('tbody');

    headers.forEach((header, index) => {
        header.addEventListener('click', () => {
            const isAsc = header.classList.contains('sorted-asc');

            // Clear all sort classes
            headers.forEach(h => h.classList.remove('sorted-asc', 'sorted-desc'));

            // Set new sort direction
            header.classList.add(isAsc ? 'sorted-desc' : 'sorted-asc');

            // Sort rows
            const rows = Array.from(tbody.querySelectorAll('tr'));
            const direction = isAsc ? -1 : 1;

            rows.sort((a, b) => {
                const aCell = a.cells[index];
                const bCell = b.cells[index];
                if (!aCell || !bCell) return 0;

                const aVal = aCell.textContent.trim();
                const bVal = bCell.textContent.trim();

                // IP Address Sort
                const isIp = (str) => /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/.test(str);

                // Helper to extract IP if it's inside a link or mixed content
                // But textContent usually returns just the text, e.g. "192.168.1.1".
                // Sometimes it might have newlines if complex. 
                // Let's assume the cell starts with an IP or is an IP.

                // Better approach: Check if it LOOKS like an IP (IPv4)
                const ipRegex = /(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})/;
                const aIpMatch = aVal.match(ipRegex);
                const bIpMatch = bVal.match(ipRegex);

                if (aIpMatch && bIpMatch) {
                    const ipToNum = (ip) => {
                        return ip.split('.').reduce((acc, octet) => (acc << 8) + parseInt(octet, 10), 0) >>> 0;
                    };
                    return (ipToNum(aIpMatch[0]) - ipToNum(bIpMatch[0])) * direction;
                }

                // Try numeric sort first
                const aNum = parseFloat(aVal);
                const bNum = parseFloat(bVal);
                if (!isNaN(aNum) && !isNaN(bNum)) {
                    return (aNum - bNum) * direction;
                }

                // Fall back to string sort
                return aVal.localeCompare(bVal) * direction;
            });

            // Re-append sorted rows
            rows.forEach(row => tbody.appendChild(row));
        });
    });
}

function escapeHtml(unsafe) {
    if (!unsafe) return '';
    return unsafe
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;")
        .replace(/'/g, "&#039;");
}

// Global debounce utility
function debounce(func, wait) {
    let timeout;
    const executedFunction = function (...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
    executedFunction.cancel = function () {
        clearTimeout(timeout);
    };
    return executedFunction;
}
