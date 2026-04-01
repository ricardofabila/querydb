// editor.js - Item editor component
// Validates: Requirements 5.1, 5.2, 5.3, 5.4, 6.1-6.6, 7.1-7.6, 8.1-8.5

const Editor = (() => {
    let currentItem = null;
    let currentMode = 'view'; // 'view' | 'edit' | 'create'
    let currentView = 'tree'; // 'tree' | 'json'

    /**
     * Show the editor panel.
     */
    function openPanel() {
        document.getElementById('item-editor').style.display = '';
    }

    /**
     * Close the editor panel.
     */
    function close() {
        document.getElementById('item-editor').style.display = 'none';
        document.getElementById('item-editor-error').style.display = 'none';
        currentItem = null;
        currentMode = 'view';
    }

    /**
     * Display an item in detail view.
     * @param {object} item
     */
    function showItemDetail(item) {
        currentItem = item;
        currentMode = 'view';
        currentView = 'tree';
        openPanel();

        document.getElementById('editor-title').textContent = 'Item Detail';
        showViewActions();
        clearError();
        renderContent();
    }

    /**
     * Render the editor content based on current mode and view.
     */
    function renderContent() {
        const content = document.getElementById('item-editor-content');
        content.innerHTML = '';

        if (currentMode === 'view') {
            // View toggle
            const toggle = createViewToggle();
            content.appendChild(toggle);

            if (currentView === 'tree') {
                const tree = document.createElement('div');
                tree.className = 'item-tree';
                renderItemTree(currentItem, tree);
                content.appendChild(tree);
            } else {
                const jsonDiv = document.createElement('div');
                jsonDiv.className = 'json-view';
                const pre = document.createElement('pre');
                pre.textContent = JSON.stringify(currentItem, null, 2);
                jsonDiv.appendChild(pre);
                content.appendChild(jsonDiv);
            }
        } else if (currentMode === 'create') {
            renderCreateForm(content);
        } else if (currentMode === 'edit') {
            renderEditForm(content);
        }
    }

    /**
     * Create the tree/json view toggle buttons.
     * @returns {HTMLElement}
     */
    function createViewToggle() {
        const div = document.createElement('div');
        div.className = 'view-toggle';

        const treeBtn = document.createElement('button');
        treeBtn.className = 'view-toggle-btn' + (currentView === 'tree' ? ' active' : '');
        treeBtn.textContent = 'Tree';
        treeBtn.addEventListener('click', () => {
            currentView = 'tree';
            renderContent();
        });

        const jsonBtn = document.createElement('button');
        jsonBtn.className = 'view-toggle-btn' + (currentView === 'json' ? ' active' : '');
        jsonBtn.textContent = 'JSON';
        jsonBtn.addEventListener('click', () => {
            currentView = 'json';
            renderContent();
        });

        div.appendChild(treeBtn);
        div.appendChild(jsonBtn);
        return div;
    }

    /**
     * Render an item as a tree structure.
     * @param {*} value
     * @param {HTMLElement} container
     * @param {string} [key]
     */
    function renderItemTree(value, container, key) {
        if (value === null || value === undefined) {
            const node = createTreeLeaf(key, 'null', 'null');
            container.appendChild(node);
            return;
        }

        if (Array.isArray(value)) {
            const node = document.createElement('div');
            node.className = 'tree-node';

            const toggle = document.createElement('button');
            toggle.className = 'tree-toggle';
            toggle.textContent = '▼';
            node.appendChild(toggle);

            if (key !== undefined) {
                const keySpan = document.createElement('span');
                keySpan.className = 'tree-key';
                keySpan.textContent = key + ':';
                node.appendChild(keySpan);
            }

            const label = document.createElement('span');
            label.className = 'tree-value';
            label.textContent = `Array (${value.length})`;
            node.appendChild(label);

            const children = document.createElement('div');
            children.className = 'tree-children';
            value.forEach((v, i) => renderItemTree(v, children, String(i)));
            node.appendChild(children);

            toggle.addEventListener('click', () => {
                const collapsed = children.style.display === 'none';
                children.style.display = collapsed ? '' : 'none';
                toggle.textContent = collapsed ? '▼' : '▶';
            });

            container.appendChild(node);
            return;
        }

        if (typeof value === 'object') {
            const node = document.createElement('div');
            node.className = 'tree-node';

            const toggle = document.createElement('button');
            toggle.className = 'tree-toggle';
            toggle.textContent = '▼';
            node.appendChild(toggle);

            if (key !== undefined) {
                const keySpan = document.createElement('span');
                keySpan.className = 'tree-key';
                keySpan.textContent = key + ':';
                node.appendChild(keySpan);
            }

            const label = document.createElement('span');
            label.className = 'tree-value';
            const keys = Object.keys(value);
            label.textContent = `Object (${keys.length})`;
            node.appendChild(label);

            const children = document.createElement('div');
            children.className = 'tree-children';
            keys.forEach(k => renderItemTree(value[k], children, k));
            node.appendChild(children);

            toggle.addEventListener('click', () => {
                const collapsed = children.style.display === 'none';
                children.style.display = collapsed ? '' : 'none';
                toggle.textContent = collapsed ? '▼' : '▶';
            });

            container.appendChild(node);
            return;
        }

        // Primitive value
        const typeClass = typeof value;
        const displayVal = typeof value === 'string' ? `"${value}"` : String(value);
        const node = createTreeLeaf(key, displayVal, typeClass);
        container.appendChild(node);
    }

    /**
     * Create a leaf node in the tree.
     */
    function createTreeLeaf(key, displayVal, typeClass) {
        const node = document.createElement('div');
        node.className = 'tree-node';

        if (key !== undefined) {
            const keySpan = document.createElement('span');
            keySpan.className = 'tree-key';
            keySpan.textContent = key + ':';
            node.appendChild(keySpan);
        }

        const valSpan = document.createElement('span');
        valSpan.className = 'tree-value ' + typeClass;
        valSpan.textContent = displayVal;
        node.appendChild(valSpan);

        return node;
    }

    /**
     * Show the create item form.
     */
    function showCreateForm() {
        currentItem = null;
        currentMode = 'create';
        openPanel();

        document.getElementById('editor-title').textContent = 'Create Item';
        showEditActions();
        clearError();
        renderContent();
    }

    /**
     * Show the edit form for the current item.
     */
    function showEditForm() {
        if (!currentItem) return;
        currentMode = 'edit';

        document.getElementById('editor-title').textContent = 'Edit Item';
        showEditActions();
        clearError();
        renderContent();
    }

    /**
     * Render the create form.
     * @param {HTMLElement} container
     */
    function renderCreateForm(container) {
        const info = document.createElement('p');
        info.style.padding = '8px 16px';
        info.style.fontSize = '12px';
        info.style.color = '#545b64';
        info.textContent = 'Enter item attributes as JSON:';
        container.appendChild(info);

        const group = document.createElement('div');
        group.className = 'form-group';

        const textarea = document.createElement('textarea');
        textarea.className = 'form-textarea';
        textarea.id = 'item-json-input';
        textarea.placeholder = '{\n  "id": "123",\n  "name": "example"\n}';
        textarea.value = '{\n  \n}';
        group.appendChild(textarea);
        container.appendChild(group);
    }

    /**
     * Render the edit form for the current item.
     * @param {HTMLElement} container
     */
    function renderEditForm(container) {
        if (!currentItem) return;

        const info = document.createElement('p');
        info.style.padding = '8px 16px';
        info.style.fontSize = '12px';
        info.style.color = '#545b64';
        info.textContent = 'Edit the item JSON. Key attributes should not be changed.';
        container.appendChild(info);

        const group = document.createElement('div');
        group.className = 'form-group';

        const textarea = document.createElement('textarea');
        textarea.className = 'form-textarea';
        textarea.id = 'item-json-input';
        textarea.style.minHeight = '200px';
        textarea.value = JSON.stringify(currentItem, null, 2);
        group.appendChild(textarea);
        container.appendChild(group);
    }

    /**
     * Save the item (create or update).
     */
    async function saveItem() {
        const configName = Tables.getSelectedConfigName();
        if (!configName) {
            showError('No table selected.');
            return;
        }

        const textarea = document.getElementById('item-json-input');
        if (!textarea) {
            showError('No input found.');
            return;
        }

        let parsed;
        try {
            parsed = JSON.parse(textarea.value);
        } catch (e) {
            showError('Invalid JSON: ' + e.message);
            return;
        }

        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
            showError('Item must be a JSON object.');
            return;
        }

        if (Object.keys(parsed).length === 0) {
            showError('Item must have at least one attribute.');
            return;
        }

        let result;
        if (currentMode === 'create') {
            result = await API.createItem(configName, parsed);
        } else if (currentMode === 'edit' && currentItem) {
            // Determine key attributes by finding attributes that haven't changed
            // For simplicity, we send the full item as an update
            // The key is derived from the original item's key-like attributes
            const key = extractKey(currentItem);
            const updates = {};
            for (const [k, v] of Object.entries(parsed)) {
                // Skip key attributes in updates
                if (key[k] !== undefined) continue;
                updates[k] = v;
            }

            if (Object.keys(updates).length === 0) {
                showError('No changes detected (key attributes cannot be updated).');
                return;
            }

            result = await API.updateItem(configName, key, updates);
        } else {
            showError('Invalid editor state.');
            return;
        }

        if (!result.success) {
            showError(result.error.message);
            return;
        }

        // Refresh browser and close editor
        Browser.refresh();
        close();
    }

    /**
     * Delete the current item with confirmation.
     */
    async function deleteItem() {
        if (!currentItem) return;

        const configName = Tables.getSelectedConfigName();
        if (!configName) {
            showError('No table selected.');
            return;
        }

        const key = extractKey(currentItem);

        // Show confirmation modal
        const confirmed = await showConfirmModal(
            'Delete Item',
            `Are you sure you want to delete this item? This action cannot be undone.`
        );

        if (!confirmed) return;

        const result = await API.deleteItem(configName, key);

        if (!result.success) {
            showError(result.error.message);
            return;
        }

        Browser.refresh();
        close();
    }

    /**
     * Extract likely key attributes from an item.
     * Uses heuristic: first 1-2 attributes, or attributes named id/pk/sk/key.
     * @param {object} item
     * @returns {object}
     */
    function extractKey(item) {
        const keys = Object.keys(item);
        const keyNames = ['id', 'pk', 'sk', 'ID', 'PK', 'SK', 'key', 'Key'];
        const found = {};

        // First try known key attribute names
        for (const k of keyNames) {
            if (item[k] !== undefined) {
                found[k] = item[k];
            }
        }

        // If we found key-like attributes, use them
        if (Object.keys(found).length > 0) {
            return found;
        }

        // Fallback: use first attribute as key
        if (keys.length > 0) {
            found[keys[0]] = item[keys[0]];
        }

        return found;
    }

    /**
     * Show a confirmation modal and return a promise.
     * @param {string} title
     * @param {string} message
     * @returns {Promise<boolean>}
     */
    function showConfirmModal(title, message) {
        return new Promise(resolve => {
            const overlay = document.getElementById('confirm-modal');
            document.getElementById('confirm-modal-title').textContent = title;
            document.getElementById('confirm-modal-message').textContent = message;
            overlay.style.display = '';

            const okBtn = document.getElementById('confirm-modal-ok');
            const cancelBtn = document.getElementById('confirm-modal-cancel');

            function cleanup() {
                overlay.style.display = 'none';
                okBtn.removeEventListener('click', onOk);
                cancelBtn.removeEventListener('click', onCancel);
            }

            function onOk() { cleanup(); resolve(true); }
            function onCancel() { cleanup(); resolve(false); }

            okBtn.addEventListener('click', onOk);
            cancelBtn.addEventListener('click', onCancel);
        });
    }

    // --- UI helpers ---

    function showViewActions() {
        const actions = document.getElementById('item-editor-actions');
        actions.style.display = '';
        document.getElementById('edit-item-btn').style.display = '';
        document.getElementById('delete-item-btn').style.display = '';
        document.getElementById('save-item-btn').style.display = 'none';
        document.getElementById('cancel-edit-btn').style.display = 'none';
    }

    function showEditActions() {
        const actions = document.getElementById('item-editor-actions');
        actions.style.display = '';
        document.getElementById('edit-item-btn').style.display = 'none';
        document.getElementById('delete-item-btn').style.display = 'none';
        document.getElementById('save-item-btn').style.display = '';
        document.getElementById('cancel-edit-btn').style.display = '';
    }

    function showError(msg) {
        const el = document.getElementById('item-editor-error');
        el.textContent = msg;
        el.style.display = '';
    }

    function clearError() {
        document.getElementById('item-editor-error').style.display = 'none';
    }

    return {
        showItemDetail,
        renderItemTree,
        showCreateForm,
        showEditForm,
        saveItem,
        deleteItem,
        close,
    };
})();
