// api.js - API client for DynamoDB Web UI
// Validates: Requirements 10.1, 10.2, 10.3, 10.4, 10.5

const API = (() => {
    const BASE = '/api';

    /**
     * Make an API request and return a structured response.
     * @param {string} url
     * @param {object} options - fetch options
     * @returns {Promise<{success: boolean, data?: any, error?: {type: string, message: string, suggestions?: string[]}}>}
     */
    async function request(url, options = {}) {
        try {
            const resp = await fetch(url, {
                headers: { 'Content-Type': 'application/json' },
                ...options,
            });

            const body = await resp.json();

            if (!resp.ok || !body.success) {
                return {
                    success: false,
                    error: body.error || {
                        type: 'error',
                        message: `Request failed with status ${resp.status}`,
                    },
                };
            }

            return { success: true, data: body.data };
        } catch (err) {
            return {
                success: false,
                error: {
                    type: 'connection',
                    message: err.message || 'Network request failed',
                    suggestions: [
                        'Check if the server is running',
                        'Verify your network connection',
                    ],
                },
            };
        }
    }

    /**
     * Fetch the list of configured tables.
     * GET /api/tables
     */
    async function getTables() {
        return request(`${BASE}/tables`);
    }

    /**
     * Fetch items for a given table configuration.
     * GET /api/tables/{configName}/items
     * @param {string} configName
     */
    async function getTableItems(configName) {
        return request(`${BASE}/tables/${encodeURIComponent(configName)}/items`);
    }

    /**
     * Create a new item in the specified table.
     * POST /api/tables/{configName}/items
     * @param {string} configName
     * @param {object} item - the item attributes
     */
    async function createItem(configName, item) {
        return request(`${BASE}/tables/${encodeURIComponent(configName)}/items`, {
            method: 'POST',
            body: JSON.stringify({ item }),
        });
    }

    /**
     * Update an existing item.
     * PUT /api/tables/{configName}/items/{key}
     * The key is JSON-encoded then URL-encoded in the path.
     * @param {string} configName
     * @param {object} key - primary key attributes
     * @param {object} updates - attributes to update
     */
    async function updateItem(configName, key, updates) {
        const encodedKey = encodeURIComponent(JSON.stringify(key));
        return request(
            `${BASE}/tables/${encodeURIComponent(configName)}/items/${encodedKey}`,
            {
                method: 'PUT',
                body: JSON.stringify({ updates }),
            }
        );
    }

    /**
     * Delete an item from the specified table.
     * DELETE /api/tables/{configName}/items/{key}
     * The key is JSON-encoded then URL-encoded in the path.
     * @param {string} configName
     * @param {object} key - primary key attributes
     */
    async function deleteItem(configName, key) {
        const encodedKey = encodeURIComponent(JSON.stringify(key));
        return request(
            `${BASE}/tables/${encodeURIComponent(configName)}/items/${encodedKey}`,
            { method: 'DELETE' }
        );
    }

    /**
     * Describe a table to get key schema, indexes, and attribute definitions.
     * GET /api/tables/{configName}/describe
     * @param {string} configName
     */
    async function describeTable(configName) {
        return request(`${BASE}/tables/${encodeURIComponent(configName)}/describe`);
    }

    /**
     * Execute a DynamoDB Query against the specified table.
     * POST /api/tables/{configName}/query
     * @param {string} configName
     * @param {object} params - query parameters (key_condition_expression, etc.)
     */
    async function queryTable(configName, params) {
        return request(`${BASE}/tables/${encodeURIComponent(configName)}/query`, {
            method: 'POST',
            body: JSON.stringify(params),
        });
    }

    /**
     * Execute a DynamoDB Scan against the specified table.
     * POST /api/tables/{configName}/scan
     * @param {string} configName
     * @param {object} params - scan parameters (filter_expression, etc.)
     */
    async function scanTable(configName, params) {
        return request(`${BASE}/tables/${encodeURIComponent(configName)}/scan`, {
            method: 'POST',
            body: JSON.stringify(params),
        });
    }

    return { getTables, getTableItems, createItem, updateItem, deleteItem, describeTable, queryTable, scanTable };
})();
