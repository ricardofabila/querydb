// tables.js - Table list component
// Validates: Requirements 3.1, 3.2, 3.3, 3.4

const Tables = (() => {
    let tables = [];
    let selectedConfigName = null;

    /**
     * Fetch tables from the API and render the list.
     */
    async function loadTables() {
        const content = document.getElementById('table-list-content');
        const loading = document.getElementById('table-list-loading');

        loading.style.display = '';
        content.innerHTML = '';
        content.appendChild(loading);

        const result = await API.getTables();

        if (!result.success) {
            loading.style.display = 'none';
            content.innerHTML = '';
            const errEl = document.createElement('div');
            errEl.className = 'error-message';
            errEl.textContent = result.error.message;
            if (result.error.suggestions && result.error.suggestions.length) {
                const ul = document.createElement('ul');
                ul.className = 'suggestions';
                result.error.suggestions.forEach(s => {
                    const li = document.createElement('li');
                    li.textContent = s;
                    ul.appendChild(li);
                });
                errEl.appendChild(ul);
            }
            content.appendChild(errEl);
            return;
        }

        tables = result.data || [];
        // Sort tables by config name for consistent display
        tables.sort((a, b) => a.config_name.localeCompare(b.config_name));
        renderTableList();
    }

    /**
     * Render the table list into the sidebar.
     */
    function renderTableList() {
        const content = document.getElementById('table-list-content');
        content.innerHTML = '';

        if (tables.length === 0) {
            const p = document.createElement('p');
            p.className = 'placeholder-text';
            p.textContent = 'No tables configured.';
            content.appendChild(p);
            return;
        }

        tables.forEach(table => {
            const btn = document.createElement('button');
            btn.className = 'table-list-item';
            if (table.config_name === selectedConfigName) {
                btn.classList.add('selected');
            }
            btn.innerHTML =
                `<span class="config-name">${escapeHtml(table.config_name)}</span>` +
                `<span class="table-name">${escapeHtml(table.table_name)}</span>`;
            btn.addEventListener('click', () => selectTable(table.config_name));
            content.appendChild(btn);
        });
    }

    /**
     * Handle table selection.
     * @param {string} configName
     */
    function selectTable(configName) {
        selectedConfigName = configName;
        renderTableList();

        // Notify browser to load items
        Browser.loadTableItems(configName);

        // Show browser action buttons
        document.getElementById('create-item-btn').style.display = '';
        document.getElementById('refresh-items-btn').style.display = '';

        // Update browser title
        const table = tables.find(t => t.config_name === configName);
        const title = table
            ? `${table.config_name} (${table.table_name})`
            : configName;
        document.getElementById('browser-title').textContent = title;

        // Close editor panel
        Editor.close();
    }

    /**
     * Get the currently selected config name.
     * @returns {string|null}
     */
    function getSelectedConfigName() {
        return selectedConfigName;
    }

    /**
     * Get table info by config name.
     * @param {string} configName
     * @returns {object|undefined}
     */
    function getTable(configName) {
        return tables.find(t => t.config_name === configName);
    }

    /**
     * Escape HTML to prevent XSS.
     */
    function escapeHtml(str) {
        const div = document.createElement('div');
        div.textContent = str;
        return div.innerHTML;
    }

    return { loadTables, renderTableList, selectTable, getSelectedConfigName, getTable, escapeHtml };
})();
