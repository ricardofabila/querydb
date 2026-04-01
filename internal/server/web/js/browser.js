// browser.js - Table browser component
// Validates: Requirements 1.1, 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 5.1, 5.2, 5.3, 5.4, 5.5, 5.6, 5.7

const Browser = (() => {
    let currentItems = [];
    let currentColumns = [];
    let currentConfigName = null;

    // Module-level table metadata storage (key schema, indexes)
    let currentMetadata = null;

    // Store the last query/scan response for pagination (used by task 7.5)
    let lastResponse = null;

    // Store the last operation params and mode for pagination replay
    let lastOperationParams = null;
    let lastOperationMode = null; // 'query' or 'scan'

    // Track selected row indices for bulk operations
    let selectedRows = new Set();

    // Accumulated scanning statistics across pagination loads
    let stats = { totalItemsReturned: 0, totalItemsScanned: 0, totalCapacityUnits: 0, pageCount: 0 };

    /**
     * Initialize data selection panel event listeners.
     * Called once when the module loads.
     */
    function initDataSelectionPanel() {
        // Operation toggle: show/hide query constraint panel
        const scanRadio = document.getElementById('operation-scan');
        const queryRadio = document.getElementById('operation-query');
        scanRadio.addEventListener('change', handleOperationToggle);
        queryRadio.addEventListener('change', handleOperationToggle);

        // Index selector: update key field labels
        document.getElementById('index-selector')
            .addEventListener('change', handleIndexChange);

        // Sort key operator: show/hide second value for "between"
        document.getElementById('sk-operator')
            .addEventListener('change', handleSkOperatorChange);

        // Filter expression builder: add filter row button
        document.getElementById('add-filter-btn')
            .addEventListener('click', addFilterRow);

        // Projection selector: show/hide attribute input
        const projAll = document.getElementById('projection-all');
        const projSpecific = document.getElementById('projection-specific');
        projAll.addEventListener('change', handleProjectionToggle);
        projSpecific.addEventListener('change', handleProjectionToggle);

        // Execute button: run query or scan
        document.getElementById('execute-btn')
            .addEventListener('click', executeOperation);
    }

    /**
     * Handle operation mode toggle (Scan/Query).
     * Shows query constraint panel only when Query is selected.
     */
    function handleOperationToggle() {
        const isQuery = document.getElementById('operation-query').checked;
        document.getElementById('query-constraint-panel').style.display = isQuery ? '' : 'none';
    }

    /**
     * Handle index selector change.
     * Updates partition key and sort key labels based on the selected index's key schema.
     */
    function handleIndexChange() {
        if (!currentMetadata) return;

        const selector = document.getElementById('index-selector');
        const selectedValue = selector.value;
        const keySchema = getKeySchemaForIndex(selectedValue);

        updateKeyLabels(keySchema);
    }

    /**
     * Get the key schema for a given index name.
     * Empty string means the base table.
     * @param {string} indexName
     * @returns {Array} key schema elements
     */
    function getKeySchemaForIndex(indexName) {
        if (!currentMetadata) return [];

        if (!indexName) {
            // Base table
            return currentMetadata.key_schema || [];
        }

        // Search GSIs
        const gsis = currentMetadata.global_secondary_indexes || [];
        const gsi = gsis.find(g => g.index_name === indexName);
        if (gsi) return gsi.key_schema || [];

        // Search LSIs
        const lsis = currentMetadata.local_secondary_indexes || [];
        const lsi = lsis.find(l => l.index_name === indexName);
        if (lsi) return lsi.key_schema || [];

        return currentMetadata.key_schema || [];
    }

    /**
     * Update the partition key and sort key labels and visibility
     * based on the provided key schema.
     * @param {Array} keySchema
     */
    function updateKeyLabels(keySchema) {
        const pkLabel = document.getElementById('pk-label');
        const skLabel = document.getElementById('sk-label');
        const skInput = document.getElementById('sk-input');
        const skOperator = document.getElementById('sk-operator');
        const skValue2 = document.getElementById('sk-value2');

        const hashKey = keySchema.find(k => k.key_type === 'HASH');
        const rangeKey = keySchema.find(k => k.key_type === 'RANGE');

        pkLabel.textContent = hashKey ? hashKey.attribute_name : 'Partition Key';

        if (rangeKey) {
            skLabel.textContent = rangeKey.attribute_name;
            skLabel.parentElement.style.display = '';
        } else {
            skLabel.textContent = 'Sort Key';
            skLabel.parentElement.style.display = 'none';
            skInput.value = '';
            skValue2.value = '';
        }
    }

    /**
     * Handle sort key operator change.
     * Shows second value input when "between" is selected.
     */
    function handleSkOperatorChange() {
        const op = document.getElementById('sk-operator').value;
        document.getElementById('sk-value2').style.display = op === 'between' ? '' : 'none';
    }

    /**
     * Handle projection mode toggle.
     * Shows the attribute name input when "Specific attributes" is selected.
     */
    function handleProjectionToggle() {
        const isSpecific = document.getElementById('projection-specific').checked;
        document.getElementById('projection-attrs').style.display = isSpecific ? '' : 'none';
    }

    /**
     * Build a projection expression from the projection selector state.
     * Returns null when "All attributes" is selected.
     * Returns { projectionExpression, expressionAttributeNames } when specific
     * attributes are provided.
     * @returns {object|null}
     */
    function buildProjectionExpression() {
        const isSpecific = document.getElementById('projection-specific').checked;
        if (!isSpecific) return null;

        const input = document.querySelector('#projection-attrs input');
        const raw = (input && input.value) || '';
        const attrs = raw.split(',').map(s => s.trim()).filter(Boolean);
        if (attrs.length === 0) return null;

        const expressionAttributeNames = {};
        const placeholders = attrs.map((attr, i) => {
            const placeholder = `#p${i}`;
            expressionAttributeNames[placeholder] = attr;
            return placeholder;
        });

        return {
            projectionExpression: placeholders.join(', '),
            expressionAttributeNames,
        };
    }

    /**
     * Collect all known attribute names from current items and table metadata
     * (key schemas from base table + all indexes).
     * @returns {string[]} sorted unique attribute names
     */
    function getKnownAttributeNames() {
        var names = new Set();

        // From currently loaded items
        currentColumns.forEach(function(c) { names.add(c); });

        if (currentMetadata) {
            // From attribute definitions
            (currentMetadata.attribute_definitions || []).forEach(function(d) {
                names.add(d.attribute_name);
            });

            // From base table key schema
            (currentMetadata.key_schema || []).forEach(function(k) {
                names.add(k.attribute_name);
            });

            // From GSI key schemas
            (currentMetadata.global_secondary_indexes || []).forEach(function(idx) {
                (idx.key_schema || []).forEach(function(k) {
                    names.add(k.attribute_name);
                });
            });

            // From LSI key schemas
            (currentMetadata.local_secondary_indexes || []).forEach(function(idx) {
                (idx.key_schema || []).forEach(function(k) {
                    names.add(k.attribute_name);
                });
            });
        }

        return Array.from(names).sort();
    }

    /**
     * Create or update the shared datalist element used for filter attribute
     * name autocomplete.
     */
    function refreshAttributeDatalist() {
        var id = 'filter-attr-datalist';
        var datalist = document.getElementById(id);
        if (!datalist) {
            datalist = document.createElement('datalist');
            datalist.id = id;
            document.body.appendChild(datalist);
        }
        datalist.innerHTML = '';
        getKnownAttributeNames().forEach(function(name) {
            var opt = document.createElement('option');
            opt.value = name;
            datalist.appendChild(opt);
        });
    }

    /** DynamoDB type options for the filter type dropdown. */
    const FILTER_TYPES = [
        'String', 'Number', 'Binary', 'Boolean', 'Null',
        'String Set', 'Number Set', 'Binary Set', 'List', 'Map'
    ];

    /** Condition options for the filter condition dropdown. */
    const FILTER_CONDITIONS = [
        'Equal to', 'Not equal to', 'Less than', 'Less than or equal',
        'Greater than', 'Greater than or equal', 'Between',
        'Exists', 'Not exists', 'Contains', 'Not contains', 'Begins with'
    ];

    /**
     * Add a filter row to the filter expression builder.
     * Each row has: attribute name input, type dropdown, condition dropdown,
     * value input, and a remove button.
     */
    function addFilterRow() {
        const container = document.getElementById('filter-rows');
        const row = document.createElement('div');
        row.className = 'filter-row';

        // Attribute name input with autocomplete from known attributes
        const nameInput = document.createElement('input');
        nameInput.type = 'text';
        nameInput.className = 'form-input filter-attr-name';
        nameInput.placeholder = 'Attribute name';
        nameInput.setAttribute('list', 'filter-attr-datalist');
        refreshAttributeDatalist();

        // Type dropdown
        const typeSelect = document.createElement('select');
        typeSelect.className = 'form-select filter-type';
        FILTER_TYPES.forEach(t => {
            const opt = document.createElement('option');
            opt.value = t;
            opt.textContent = t;
            typeSelect.appendChild(opt);
        });

        // Condition dropdown
        const condSelect = document.createElement('select');
        condSelect.className = 'form-select filter-condition';
        FILTER_CONDITIONS.forEach(c => {
            const opt = document.createElement('option');
            opt.value = c;
            opt.textContent = c;
            condSelect.appendChild(opt);
        });

        // Value input
        const valueInput = document.createElement('input');
        valueInput.type = 'text';
        valueInput.className = 'form-input filter-value';
        valueInput.placeholder = 'Value';

        // Disable value input when condition is Exists/Not exists
        condSelect.addEventListener('change', () => {
            const cond = condSelect.value;
            const disabled = cond === 'Exists' || cond === 'Not exists';
            valueInput.disabled = disabled;
            if (disabled) valueInput.value = '';
        });

        // Remove button
        const removeBtn = document.createElement('button');
        removeBtn.type = 'button';
        removeBtn.className = 'btn btn-icon filter-remove-btn';
        removeBtn.textContent = '✕';
        removeBtn.title = 'Remove filter';
        removeBtn.addEventListener('click', () => row.remove());

        row.appendChild(nameInput);
        row.appendChild(typeSelect);
        row.appendChild(condSelect);
        row.appendChild(valueInput);
        row.appendChild(removeBtn);
        container.appendChild(row);
    }

    /**
     * Map a human-readable type name to a DynamoDB type code.
     * @param {string} typeName
     * @returns {string}
     */
    function mapFilterTypeToDDBType(typeName) {
        const map = {
            'String': 'S', 'Number': 'N', 'Binary': 'B',
            'Boolean': 'BOOL', 'Null': 'NULL',
            'String Set': 'SS', 'Number Set': 'NS', 'Binary Set': 'BS',
            'List': 'L', 'Map': 'M'
        };
        return map[typeName] || 'S';
    }

    /**
     * Build a typed value object for a filter expression attribute value.
     * @param {string} rawValue - the raw string from the value input
     * @param {string} ddbType - the DynamoDB type code
     * @returns {object} TypedValue {value, type}
     */
    function buildTypedFilterValue(rawValue, ddbType) {
        switch (ddbType) {
            case 'N':
                return { value: rawValue, type: 'N' };
            case 'BOOL':
                return { value: rawValue.toLowerCase() === 'true', type: 'BOOL' };
            case 'NULL':
                return { value: null, type: 'NULL' };
            default:
                return { value: rawValue, type: ddbType };
        }
    }

    /**
     * Build a DynamoDB filter expression from the current filter rows.
     * Returns an object with filterExpression, expressionAttributeNames,
     * and expressionAttributeValues, or null if no valid filters exist.
     * @returns {object|null}
     */
    function buildFilterExpression() {
        const rows = document.querySelectorAll('#filter-rows .filter-row');
        if (rows.length === 0) return null;

        const conditions = [];
        const exprAttrNames = {};
        const exprAttrValues = {};

        rows.forEach((row, i) => {
            const attrName = row.querySelector('.filter-attr-name').value.trim();
            if (!attrName) return;

            const typeName = row.querySelector('.filter-type').value;
            const condition = row.querySelector('.filter-condition').value;
            const rawValue = row.querySelector('.filter-value').value;
            const ddbType = mapFilterTypeToDDBType(typeName);

            const nameKey = `#f${i}`;
            const valueKey = `:f${i}`;
            exprAttrNames[nameKey] = attrName;

            let expr;
            switch (condition) {
                case 'Equal to':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} = ${valueKey}`;
                    break;
                case 'Not equal to':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} <> ${valueKey}`;
                    break;
                case 'Less than':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} < ${valueKey}`;
                    break;
                case 'Less than or equal':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} <= ${valueKey}`;
                    break;
                case 'Greater than':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} > ${valueKey}`;
                    break;
                case 'Greater than or equal':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `${nameKey} >= ${valueKey}`;
                    break;
                case 'Between': {
                    const parts = rawValue.split(',').map(s => s.trim());
                    const loKey = `:f${i}lo`;
                    const hiKey = `:f${i}hi`;
                    exprAttrValues[loKey] = buildTypedFilterValue(parts[0] || '', ddbType);
                    exprAttrValues[hiKey] = buildTypedFilterValue(parts[1] || '', ddbType);
                    expr = `${nameKey} BETWEEN ${loKey} AND ${hiKey}`;
                    break;
                }
                case 'Exists':
                    expr = `attribute_exists(${nameKey})`;
                    break;
                case 'Not exists':
                    expr = `attribute_not_exists(${nameKey})`;
                    break;
                case 'Contains':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `contains(${nameKey}, ${valueKey})`;
                    break;
                case 'Not contains':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `NOT contains(${nameKey}, ${valueKey})`;
                    break;
                case 'Begins with':
                    exprAttrValues[valueKey] = buildTypedFilterValue(rawValue, ddbType);
                    expr = `begins_with(${nameKey}, ${valueKey})`;
                    break;
                default:
                    return;
            }

            conditions.push(expr);
        });

        if (conditions.length === 0) return null;

        return {
            filterExpression: conditions.join(' AND '),
            expressionAttributeNames: exprAttrNames,
            expressionAttributeValues: exprAttrValues,
        };
    }

    /**
     * Populate the index selector dropdown from table metadata.
     * @param {object} metadata - TableMetadata from describeTable
     */
    function populateIndexSelector(metadata) {
        const selector = document.getElementById('index-selector');
        selector.innerHTML = '';

        // Base Table option
        const baseOpt = document.createElement('option');
        baseOpt.value = '';
        baseOpt.textContent = 'Base Table';
        selector.appendChild(baseOpt);

        // GSIs
        const gsis = metadata.global_secondary_indexes || [];
        gsis.forEach(gsi => {
            const opt = document.createElement('option');
            opt.value = gsi.index_name;
            opt.textContent = `GSI: ${gsi.index_name}`;
            selector.appendChild(opt);
        });

        // LSIs
        const lsis = metadata.local_secondary_indexes || [];
        lsis.forEach(lsi => {
            const opt = document.createElement('option');
            opt.value = lsi.index_name;
            opt.textContent = `LSI: ${lsi.index_name}`;
            selector.appendChild(opt);
        });
    }

    /**
     * Reset the data selection panel to defaults.
     */
    function resetDataSelectionPanel() {
        document.getElementById('operation-scan').checked = true;
        document.getElementById('query-constraint-panel').style.display = 'none';
        document.getElementById('pk-input').value = '';
        document.getElementById('sk-input').value = '';
        document.getElementById('sk-value2').value = '';
        document.getElementById('sk-value2').style.display = 'none';
        document.getElementById('sk-operator').value = '=';
        // Clear all filter rows
        document.getElementById('filter-rows').innerHTML = '';
        // Reset projection selector
        document.getElementById('projection-all').checked = true;
        document.getElementById('projection-specific').checked = false;
        document.getElementById('projection-attrs').style.display = 'none';
        const projInput = document.querySelector('#projection-attrs input');
        if (projInput) projInput.value = '';
    }

    /**
     * Validate that partition key is present before query execution.
     * @returns {boolean} true if valid
     */
    function validateQueryConstraints() {
        const isQuery = document.getElementById('operation-query').checked;
        if (!isQuery) return true;

        const pkInput = document.getElementById('pk-input');
        if (!pkInput.value.trim()) {
            pkInput.focus();
            App.showGlobalError('Partition key value is required for Query operations.');
            return false;
        }
        return true;
    }

    /**
     * Look up the DynamoDB attribute type for a key attribute name
     * from the table metadata attribute_definitions.
     * @param {string} attrName
     * @returns {string} DynamoDB type code (e.g. 'S', 'N', 'B'), defaults to 'S'
     */
    function getAttributeType(attrName) {
        if (!currentMetadata || !currentMetadata.attribute_definitions) return 'S';
        const def = currentMetadata.attribute_definitions.find(
            d => d.attribute_name === attrName
        );
        return def ? def.attribute_type : 'S';
    }

    /**
     * Execute a query or scan operation based on the current panel state.
     * Validates constraints, assembles parameters, calls the API,
     * and renders results.
     */
    async function executeOperation() {
        // Validate query constraints (pk required for Query mode)
        if (!validateQueryConstraints()) return;
        if (!currentConfigName) return;

        const isQuery = document.getElementById('operation-query').checked;
        const indexName = document.getElementById('index-selector').value;
        const limitInput = document.getElementById('page-limit');
        const limit = parseInt(limitInput.value, 10) || 50;

        // Build filter expression
        const filterResult = buildFilterExpression();

        // Build projection expression
        const projResult = buildProjectionExpression();

        // Merge expression attribute names from filter and projection
        const expressionAttributeNames = {};
        const expressionAttributeValues = {};

        if (filterResult) {
            Object.assign(expressionAttributeNames, filterResult.expressionAttributeNames);
            Object.assign(expressionAttributeValues, filterResult.expressionAttributeValues);
        }
        if (projResult) {
            Object.assign(expressionAttributeNames, projResult.expressionAttributeNames);
        }

        // Assemble request body
        const params = {
            limit: limit,
        };

        if (indexName) {
            params.index_name = indexName;
        }
        if (filterResult) {
            params.filter_expression = filterResult.filterExpression;
        }
        if (projResult) {
            params.projection_expression = projResult.projectionExpression;
        }
        if (Object.keys(expressionAttributeNames).length > 0) {
            params.expression_attribute_names = expressionAttributeNames;
        }
        if (Object.keys(expressionAttributeValues).length > 0) {
            params.expression_attribute_values = expressionAttributeValues;
        }

        // For Query mode: build key condition expression
        if (isQuery) {
            const keySchema = getKeySchemaForIndex(indexName);
            const hashKey = keySchema.find(k => k.key_type === 'HASH');
            const rangeKey = keySchema.find(k => k.key_type === 'RANGE');

            const pkValue = document.getElementById('pk-input').value.trim();
            const pkAttrName = hashKey ? hashKey.attribute_name : 'pk';
            const pkType = getAttributeType(pkAttrName);

            // Always add pk name/value placeholders
            expressionAttributeNames['#pk'] = pkAttrName;
            expressionAttributeValues[':pkval'] = { value: pkValue, type: pkType };

            let keyCondition = '#pk = :pkval';

            // Sort key condition (if sort key exists and has a value)
            const skValue = document.getElementById('sk-input').value.trim();
            if (rangeKey && skValue) {
                const skOperator = document.getElementById('sk-operator').value;
                const skAttrName = rangeKey.attribute_name;
                const skType = getAttributeType(skAttrName);

                expressionAttributeNames['#sk'] = skAttrName;

                if (skOperator === 'between') {
                    const skValue2 = document.getElementById('sk-value2').value.trim();
                    expressionAttributeValues[':sklo'] = { value: skValue, type: skType };
                    expressionAttributeValues[':skhi'] = { value: skValue2, type: skType };
                    keyCondition += ' AND #sk BETWEEN :sklo AND :skhi';
                } else if (skOperator === 'begins_with') {
                    expressionAttributeValues[':skval'] = { value: skValue, type: skType };
                    keyCondition += ' AND begins_with(#sk, :skval)';
                } else {
                    // Comparison operators: =, <, <=, >, >=
                    expressionAttributeValues[':skval'] = { value: skValue, type: skType };
                    keyCondition += ` AND #sk ${skOperator} :skval`;
                }
            }

            params.key_condition_expression = keyCondition;
            // Re-assign merged names/values (key placeholders were added above)
            params.expression_attribute_names = expressionAttributeNames;
            params.expression_attribute_values = expressionAttributeValues;
        }

        // Store operation params and mode for pagination replay
        lastOperationParams = Object.assign({}, params);
        lastOperationMode = isQuery ? 'query' : 'scan';

        // Show loading state
        const content = document.getElementById('table-browser-content');
        const loading = document.getElementById('table-browser-loading');
        const errorEl = document.getElementById('table-browser-error');
        const executeBtn = document.getElementById('execute-btn');

        content.innerHTML = '';
        errorEl.style.display = 'none';
        loading.style.display = '';
        executeBtn.disabled = true;
        executeBtn.textContent = 'Executing…';

        try {
            const result = isQuery
                ? await API.queryTable(currentConfigName, params)
                : await API.scanTable(currentConfigName, params);

            loading.style.display = 'none';
            executeBtn.disabled = false;
            executeBtn.textContent = 'Execute';

            if (!result.success) {
                errorEl.style.display = '';
                errorEl.textContent = result.error.message || 'Operation failed';
                lastResponse = null;
                currentItems = [];
                currentColumns = [];
                return;
            }

            // Store the response for pagination (task 7.5)
            lastResponse = result.data;

            // Reset and set initial statistics (task 7.6)
            stats = {
                totalItemsReturned: result.data.count || 0,
                totalItemsScanned: result.data.scanned_count || 0,
                totalCapacityUnits: result.data.consumed_capacity ? result.data.consumed_capacity.capacity_units : 0,
                pageCount: 1,
            };

            // Store typed items and render
            currentItems = (result.data && result.data.items) || [];
            currentColumns = extractColumns(currentItems);
            renderTableBrowser();
        } catch (err) {
            loading.style.display = 'none';
            executeBtn.disabled = false;
            executeBtn.textContent = 'Execute';
            errorEl.style.display = '';
            errorEl.textContent = err.message || 'Unexpected error';
            lastResponse = null;
            currentItems = [];
            currentColumns = [];
        }
    }

    /**
     * Get the last query/scan response (for pagination).
     * @returns {object|null}
     */
    function getLastResponse() {
        return lastResponse;
    }

    /**
     * Load more items using the LastEvaluatedKey from the previous response.
     * Appends new items to the existing result set and re-renders the table.
     * Validates: Requirements 12.1, 12.2, 12.3, 12.4
     */
    async function loadMoreItems() {
        if (!lastResponse || !lastResponse.last_evaluated_key) return;
        if (!lastOperationParams || !lastOperationMode || !currentConfigName) return;

        // Build follow-up params with exclusive_start_key
        const params = Object.assign({}, lastOperationParams, {
            exclusive_start_key: lastResponse.last_evaluated_key,
        });

        // Disable the load-more button while loading
        const loadMoreBtn = document.getElementById('load-more-btn');
        if (loadMoreBtn) {
            loadMoreBtn.disabled = true;
            loadMoreBtn.textContent = 'Loading…';
        }

        try {
            const result = lastOperationMode === 'query'
                ? await API.queryTable(currentConfigName, params)
                : await API.scanTable(currentConfigName, params);

            if (!result.success) {
                if (loadMoreBtn) {
                    loadMoreBtn.disabled = false;
                    loadMoreBtn.textContent = 'Load more';
                }
                App.showGlobalError(result.error.message || 'Failed to load more items.');
                return;
            }

            // Update lastResponse for next pagination
            lastResponse = result.data;

            // Accumulate statistics across pagination loads (task 7.6)
            stats.totalItemsReturned += result.data.count || 0;
            stats.totalItemsScanned += result.data.scanned_count || 0;
            stats.totalCapacityUnits += result.data.consumed_capacity ? result.data.consumed_capacity.capacity_units : 0;
            stats.pageCount += 1;

            // Append new items to existing results
            const newItems = (result.data && result.data.items) || [];
            currentItems = currentItems.concat(newItems);
            currentColumns = extractColumns(currentItems);
            renderTableBrowser();
        } catch (err) {
            if (loadMoreBtn) {
                loadMoreBtn.disabled = false;
                loadMoreBtn.textContent = 'Load more';
            }
            App.showGlobalError(err.message || 'Failed to load more items.');
        }
    }

    /**
     * Fetch and display items for a table.
     * Also calls describeTable to populate the data selection panel.
     * @param {string} configName
     */
    async function loadTableItems(configName) {
        currentConfigName = configName;
        const content = document.getElementById('table-browser-content');
        const loading = document.getElementById('table-browser-loading');
        const errorEl = document.getElementById('table-browser-error');

        content.innerHTML = '';
        errorEl.style.display = 'none';
        loading.style.display = '';

        // Reset pagination state for new table load
        lastResponse = null;
        lastOperationParams = null;
        lastOperationMode = null;

        // Show the data selection panel
        document.getElementById('data-selection-panel').style.display = '';

        // Reset panel to defaults
        resetDataSelectionPanel();

        // Fetch table metadata for the data selection panel
        const metaResult = await API.describeTable(configName);
        if (metaResult.success) {
            currentMetadata = metaResult.data;
            populateIndexSelector(currentMetadata);
            // Set initial key labels from base table key schema
            updateKeyLabels(currentMetadata.key_schema || []);
            refreshAttributeDatalist();
        } else {
            currentMetadata = null;
        }

        // Fetch items (existing behavior)
        const result = await API.getTableItems(configName);

        loading.style.display = 'none';

        if (!result.success) {
            errorEl.style.display = '';
            errorEl.innerHTML = Tables.escapeHtml(result.error.message);
            if (result.error.suggestions && result.error.suggestions.length) {
                const ul = document.createElement('ul');
                ul.className = 'suggestions';
                result.error.suggestions.forEach(s => {
                    const li = document.createElement('li');
                    li.textContent = s;
                    ul.appendChild(li);
                });
                errorEl.appendChild(ul);
            }
            currentItems = [];
            currentColumns = [];
            return;
        }

        currentItems = (result.data && result.data.items) || [];
        currentColumns = extractColumns(currentItems);
        refreshAttributeDatalist();
        renderTableBrowser();
    }

    /**
     * Extract unique column names from all items, placing key attributes first.
     * Handles both typed items ({value, type} format from query/scan) and
     * untyped items (plain values from initial getTableItems load).
     * @param {Array} items
     * @returns {string[]}
     */
    function extractColumns(items) {
        const colSet = new Set();
        items.forEach(item => {
            Object.keys(item).forEach(k => colSet.add(k));
        });

        // Determine key column names from metadata key schema
        const keyColumns = [];
        if (currentMetadata && currentMetadata.key_schema) {
            const hashKey = currentMetadata.key_schema.find(k => k.key_type === 'HASH');
            const rangeKey = currentMetadata.key_schema.find(k => k.key_type === 'RANGE');
            if (hashKey) keyColumns.push(hashKey.attribute_name);
            if (rangeKey) keyColumns.push(rangeKey.attribute_name);
        }

        const keySet = new Set(keyColumns);
        const nonKeyCols = Array.from(colSet)
            .filter(c => !keySet.has(c))
            .sort((a, b) => a.localeCompare(b));

        // Key columns first (in HASH, RANGE order), then remaining sorted alphabetically
        return [...keyColumns.filter(k => colSet.has(k)), ...nonKeyCols];
    }

    /**
     * Determine if a value is a typed item value ({value, type} format).
     * @param {*} val
     * @returns {boolean}
     */
    function isTypedValue(val) {
        return val !== null && typeof val === 'object' && !Array.isArray(val)
            && 'value' in val && 'type' in val;
    }

    /**
     * Extract the display value from an item's attribute.
     * For typed items (from query/scan), returns item[col].value.
     * For untyped items (from initial load), returns item[col] directly.
     * @param {*} cellVal - the raw value from item[col]
     * @returns {*} the display value
     */
    function getCellDisplayValue(cellVal) {
        if (isTypedValue(cellVal)) {
            return cellVal.value;
        }
        return cellVal;
    }

    /**
     * Render the items table in the browser panel.
     * Handles both typed items ({value, type} from query/scan) and
     * untyped items (plain values from initial load).
     * Includes a checkbox column for bulk selection.
     */
    function renderTableBrowser() {
        const content = document.getElementById('table-browser-content');
        content.innerHTML = '';

        // Reset selection state
        selectedRows = new Set();

        // Toolbar with create item button (always shown when a table is loaded)
        if (currentConfigName) {
            const toolbar = document.createElement('div');
            toolbar.className = 'result-table-toolbar';

            const createBtn = document.createElement('button');
            createBtn.className = 'btn btn-primary btn-icon';
            createBtn.textContent = '+';
            createBtn.title = 'Create item';
            createBtn.addEventListener('click', openCreateItemModal);
            toolbar.appendChild(createBtn);

            // Export controls
            const exportContainer = document.createElement('div');
            exportContainer.className = 'export-controls';

            const exportBtn = document.createElement('button');
            exportBtn.className = 'btn btn-secondary';
            exportBtn.textContent = 'Export ▾';
            exportBtn.addEventListener('click', () => {
                dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none';
            });

            const dropdown = document.createElement('div');
            dropdown.className = 'export-dropdown';
            dropdown.style.display = 'none';

            const csvBtn = document.createElement('button');
            csvBtn.textContent = 'Download as CSV';
            csvBtn.addEventListener('click', () => {
                Export.downloadCSV(currentItems, currentColumns, currentConfigName);
                dropdown.style.display = 'none';
            });

            const jsonBtn = document.createElement('button');
            jsonBtn.textContent = 'Download as JSON';
            jsonBtn.addEventListener('click', () => {
                Export.downloadJSON(currentItems, currentConfigName);
                dropdown.style.display = 'none';
            });

            dropdown.appendChild(csvBtn);
            dropdown.appendChild(jsonBtn);
            exportContainer.appendChild(exportBtn);
            exportContainer.appendChild(dropdown);
            toolbar.appendChild(exportContainer);

            content.appendChild(toolbar);
        }

        // Bulk action bar (hidden by default, shown when rows are selected)
        const bulkBar = document.createElement('div');
        bulkBar.id = 'bulk-action-bar';
        bulkBar.className = 'bulk-action-bar';
        bulkBar.style.display = 'none';

        const bulkCount = document.createElement('span');
        bulkCount.className = 'bulk-count';
        bulkCount.textContent = '0 selected';
        bulkBar.appendChild(bulkCount);

        const deleteBtn = document.createElement('button');
        deleteBtn.className = 'btn btn-danger';
        deleteBtn.textContent = 'Delete';
        deleteBtn.addEventListener('click', bulkDelete);
        bulkBar.appendChild(deleteBtn);

        const duplicateBtn = document.createElement('button');
        duplicateBtn.className = 'btn btn-secondary';
        duplicateBtn.textContent = 'Duplicate';
        duplicateBtn.addEventListener('click', bulkDuplicate);
        bulkBar.appendChild(duplicateBtn);

        content.appendChild(bulkBar);

        if (currentItems.length === 0) {
            const p = document.createElement('p');
            p.className = 'placeholder-text';
            p.textContent = 'No items in this table.';
            content.appendChild(p);
            return;
        }

        const wrapper = document.createElement('div');
        wrapper.className = 'items-table-wrapper';

        const table = document.createElement('table');
        table.className = 'items-table';

        // Header
        const thead = document.createElement('thead');
        const headerRow = document.createElement('tr');

        // Select-all checkbox header
        const selectAllTh = document.createElement('th');
        const selectAllCb = document.createElement('input');
        selectAllCb.type = 'checkbox';
        selectAllCb.title = 'Select all';
        selectAllCb.addEventListener('change', () => {
            const checked = selectAllCb.checked;
            const checkboxes = tbody.querySelectorAll('input[type="checkbox"]');
            checkboxes.forEach((cb, idx) => {
                cb.checked = checked;
                if (checked) {
                    selectedRows.add(idx);
                } else {
                    selectedRows.delete(idx);
                }
            });
            updateBulkActionBar();
        });
        selectAllTh.appendChild(selectAllCb);
        headerRow.appendChild(selectAllTh);

        currentColumns.forEach(col => {
            const th = document.createElement('th');
            th.textContent = col;
            headerRow.appendChild(th);
        });
        thead.appendChild(headerRow);
        table.appendChild(thead);

        // Body
        const tbody = document.createElement('tbody');
        currentItems.forEach((item, rowIndex) => {
            const tr = document.createElement('tr');

            // Checkbox cell
            const checkTd = document.createElement('td');
            const cb = document.createElement('input');
            cb.type = 'checkbox';
            cb.addEventListener('change', () => {
                if (cb.checked) {
                    selectedRows.add(rowIndex);
                } else {
                    selectedRows.delete(rowIndex);
                    // Uncheck select-all if any row is unchecked
                    selectAllCb.checked = false;
                }
                updateBulkActionBar();
            });
            // Stop checkbox click from triggering row click
            cb.addEventListener('click', (e) => e.stopPropagation());
            checkTd.appendChild(cb);
            tr.appendChild(checkTd);

            tr.addEventListener('click', () => {
                // Deselect previous
                const prev = tbody.querySelector('tr.selected');
                if (prev) prev.classList.remove('selected');
                tr.classList.add('selected');
                Editor.showItemDetail(item);
            });

            currentColumns.forEach(col => {
                const td = document.createElement('td');
                const rawVal = item[col];

                if (rawVal === undefined) {
                    // Missing attribute — show empty cell indicator
                    const emptySpan = document.createElement('span');
                    emptySpan.className = 'cell-value cell-complex';
                    emptySpan.textContent = '—';
                    td.appendChild(emptySpan);
                } else {
                    const displayVal = getCellDisplayValue(rawVal);
                    td.appendChild(renderCellValue(displayVal));

                    // Add type badge for typed items ({value, type} format)
                    if (isTypedValue(rawVal)) {
                        const badge = document.createElement('span');
                        badge.className = 'type-badge type-' + rawVal.type;
                        badge.textContent = rawVal.type;
                        td.appendChild(badge);
                    }

                    // Inline editing for non-key scalar cells (S, N, BOOL)
                    if (isTypedValue(rawVal) && isEditableScalar(rawVal.type) && !isKeyAttribute(col)) {
                        td.style.cursor = 'pointer';
                        td.addEventListener('click', (e) => {
                            e.stopPropagation();
                            activateInlineEditor(td, item, col, rawVal, rowIndex);
                        });
                    }

                    // JSON modal for complex types (M, L)
                    if (isTypedValue(rawVal) && (rawVal.type === 'M' || rawVal.type === 'L')) {
                        td.style.cursor = 'pointer';
                        td.addEventListener('click', (e) => {
                            e.stopPropagation();
                            openJsonEditModal(item, col, rawVal, rowIndex);
                        });
                    }
                }

                tr.appendChild(td);
            });

            tbody.appendChild(tr);
        });
        table.appendChild(tbody);
        wrapper.appendChild(table);
        content.appendChild(wrapper);

        // Item count footer
        const countEl = document.createElement('div');
        countEl.className = 'items-count';
        countEl.textContent = `${currentItems.length} item${currentItems.length !== 1 ? 's' : ''}`;
        content.appendChild(countEl);

        // Statistics display (task 7.6) — only shown after an execute operation
        if (stats.pageCount > 0) {
            const statsEl = document.createElement('div');
            statsEl.className = 'stats-display';
            statsEl.textContent = `Items returned: ${stats.totalItemsReturned} | Items scanned: ${stats.totalItemsScanned} | Capacity units: ${stats.totalCapacityUnits} | Pages: ${stats.pageCount}`;
            content.appendChild(statsEl);
        }

        // Pagination: show "Load more" button when last_evaluated_key is present
        if (lastResponse && lastResponse.last_evaluated_key) {
            const loadMoreBtn = document.createElement('button');
            loadMoreBtn.id = 'load-more-btn';
            loadMoreBtn.className = 'btn btn-secondary';
            loadMoreBtn.textContent = 'Load more';
            loadMoreBtn.addEventListener('click', loadMoreItems);
            content.appendChild(loadMoreBtn);
        }
    }

    /**
     * Check if a DynamoDB type code is an editable scalar type.
     * @param {string} type - DynamoDB type code
     * @returns {boolean}
     */
    function isEditableScalar(type) {
        return type === 'S' || type === 'N' || type === 'BOOL';
    }

    /**
     * Check if a column name is a key attribute (partition key or sort key).
     * @param {string} colName
     * @returns {boolean}
     */
    function isKeyAttribute(colName) {
        if (!currentMetadata || !currentMetadata.key_schema) return false;
        return currentMetadata.key_schema.some(k => k.attribute_name === colName);
    }

    /**
     * Extract the untyped key object from a typed item for use with the UpdateItem API.
     * @param {object} item - the typed item
     * @returns {object} untyped key object (e.g. {"pk": "user-1", "sk": "PROFILE"})
     */
    function extractItemKey(item) {
        if (!currentMetadata || !currentMetadata.key_schema) return {};
        const key = {};
        currentMetadata.key_schema.forEach(ks => {
            const attrName = ks.attribute_name;
            const rawVal = item[attrName];
            if (rawVal !== undefined) {
                key[attrName] = getCellDisplayValue(rawVal);
            }
        });
        return key;
    }

    /**
     * Activate inline editing on a table cell for scalar types (S, N, BOOL).
     * @param {HTMLElement} td - the table cell element
     * @param {object} item - the full item object
     * @param {string} col - the attribute/column name
     * @param {object} rawVal - the typed value {value, type}
     * @param {number} rowIndex - the row index in currentItems
     */
    function activateInlineEditor(td, item, col, rawVal, rowIndex) {
        // Prevent double-activation
        if (td.querySelector('.inline-editor')) return;

        const originalValue = rawVal.value;
        const originalType = rawVal.type;

        // Save original cell HTML for revert
        const originalHTML = td.innerHTML;

        // Clear cell and insert input
        td.innerHTML = '';
        const input = document.createElement('input');
        input.type = 'text';
        input.className = 'inline-editor';
        input.value = originalType === 'BOOL' ? String(originalValue) : String(originalValue);
        td.appendChild(input);

        input.focus();
        input.select();

        let committed = false;

        function commitEdit() {
            if (committed) return;
            committed = true;

            const newRawValue = input.value;

            // Convert to the appropriate untyped value for the UpdateItem API
            let untypedValue;
            if (originalType === 'N') {
                untypedValue = Number(newRawValue);
                if (isNaN(untypedValue)) {
                    // Invalid number — revert
                    td.innerHTML = originalHTML;
                    App.showGlobalError('Invalid number value.');
                    return;
                }
            } else if (originalType === 'BOOL') {
                const lower = newRawValue.toLowerCase();
                if (lower !== 'true' && lower !== 'false') {
                    td.innerHTML = originalHTML;
                    App.showGlobalError('Boolean value must be "true" or "false".');
                    return;
                }
                untypedValue = lower === 'true';
            } else {
                // String type
                untypedValue = newRawValue;
            }

            // If value hasn't changed, just revert display
            if (String(untypedValue) === String(originalValue)) {
                td.innerHTML = originalHTML;
                return;
            }

            // Build key and updates
            const key = extractItemKey(item);
            const updates = { [col]: untypedValue };

            // Send update request
            API.updateItem(currentConfigName, key, updates).then(result => {
                if (result.success) {
                    // Update the in-memory item
                    let newTypedValue;
                    if (originalType === 'N') {
                        newTypedValue = { value: String(untypedValue), type: 'N' };
                    } else if (originalType === 'BOOL') {
                        newTypedValue = { value: untypedValue, type: 'BOOL' };
                    } else {
                        newTypedValue = { value: untypedValue, type: 'S' };
                    }
                    currentItems[rowIndex][col] = newTypedValue;

                    // Re-render the cell content
                    td.innerHTML = '';
                    const displayVal = getCellDisplayValue(newTypedValue);
                    td.appendChild(renderCellValue(displayVal));

                    const badge = document.createElement('span');
                    badge.className = 'type-badge type-' + newTypedValue.type;
                    badge.textContent = newTypedValue.type;
                    td.appendChild(badge);

                    // Re-attach click handler for further edits
                    td.addEventListener('click', (e) => {
                        e.stopPropagation();
                        activateInlineEditor(td, currentItems[rowIndex], col, newTypedValue, rowIndex);
                    });
                } else {
                    // Revert on failure
                    td.innerHTML = originalHTML;
                    App.showGlobalError(result.error.message || 'Failed to update item.');
                }
            }).catch(() => {
                td.innerHTML = originalHTML;
                App.showGlobalError('Failed to update item.');
            });
        }

        function cancelEdit() {
            if (committed) return;
            committed = true;
            td.innerHTML = originalHTML;
        }

        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                commitEdit();
            } else if (e.key === 'Escape') {
                e.preventDefault();
                cancelEdit();
            }
        });

        input.addEventListener('blur', () => {
            // Small delay to allow keydown to fire first
            setTimeout(() => {
                if (!committed) commitEdit();
            }, 0);
        });
    }

    /**
     * Open the JSON edit modal for a Map or List typed value.
     * Validates: Requirements 11.1, 11.2, 11.3, 11.4, 11.5
     * @param {object} item - the full item object
     * @param {string} col - the attribute/column name
     * @param {object} rawVal - the typed value {value, type}
     * @param {number} rowIndex - the row index in currentItems
     */
    function openJsonEditModal(item, col, rawVal, rowIndex) {
        const overlay = document.getElementById('json-edit-modal');
        const title = document.getElementById('json-edit-title');
        const textarea = document.getElementById('json-edit-textarea');
        const errorEl = document.getElementById('json-edit-error');
        const saveBtn = document.getElementById('json-edit-save');
        const cancelBtn = document.getElementById('json-edit-cancel');

        title.textContent = 'Edit ' + col;
        errorEl.style.display = 'none';
        errorEl.textContent = '';
        textarea.value = JSON.stringify(rawVal.value, null, 2);
        overlay.style.display = '';
        textarea.focus();

        function cleanup() {
            overlay.style.display = 'none';
            saveBtn.removeEventListener('click', onSave);
            cancelBtn.removeEventListener('click', onCancel);
        }

        function onCancel() {
            cleanup();
        }

        async function onSave() {
            errorEl.style.display = 'none';

            let parsed;
            try {
                parsed = JSON.parse(textarea.value);
            } catch (e) {
                errorEl.textContent = 'Invalid JSON: ' + e.message;
                errorEl.style.display = '';
                return;
            }

            // Build key and update payload
            const key = extractItemKey(item);
            const updates = { [col]: parsed };

            saveBtn.disabled = true;
            saveBtn.textContent = 'Saving…';

            try {
                const result = await API.updateItem(currentConfigName, key, updates);
                saveBtn.disabled = false;
                saveBtn.textContent = 'Save';

                if (result.success) {
                    // Update in-memory item with new typed value
                    currentItems[rowIndex][col] = { value: parsed, type: rawVal.type };
                    renderTableBrowser();
                    cleanup();
                } else {
                    errorEl.textContent = result.error.message || 'Failed to update item.';
                    errorEl.style.display = '';
                }
            } catch (err) {
                saveBtn.disabled = false;
                saveBtn.textContent = 'Save';
                errorEl.textContent = err.message || 'Failed to update item.';
                errorEl.style.display = '';
            }
        }

        saveBtn.addEventListener('click', onSave);
        cancelBtn.addEventListener('click', onCancel);
    }

    /**
     * Open the JSON edit modal to create a new item.
     * Pre-fills a JSON template with key attribute names from metadata.
     * Validates: Requirements 14.1, 14.2, 14.3, 14.4, 14.5
     */
    function openCreateItemModal() {
        const overlay = document.getElementById('json-edit-modal');
        const title = document.getElementById('json-edit-title');
        const textarea = document.getElementById('json-edit-textarea');
        const errorEl = document.getElementById('json-edit-error');
        const saveBtn = document.getElementById('json-edit-save');
        const cancelBtn = document.getElementById('json-edit-cancel');

        title.textContent = 'Create Item';
        errorEl.style.display = 'none';
        errorEl.textContent = '';

        // Build a JSON template with key attribute names from metadata
        const template = {};
        if (currentMetadata && currentMetadata.key_schema) {
            currentMetadata.key_schema.forEach(ks => {
                template[ks.attribute_name] = '';
            });
        }
        textarea.value = JSON.stringify(template, null, 2);
        overlay.style.display = '';
        textarea.focus();

        function cleanup() {
            overlay.style.display = 'none';
            saveBtn.removeEventListener('click', onSave);
            cancelBtn.removeEventListener('click', onCancel);
        }

        function onCancel() {
            cleanup();
        }

        async function onSave() {
            errorEl.style.display = 'none';

            let parsed;
            try {
                parsed = JSON.parse(textarea.value);
            } catch (e) {
                errorEl.textContent = 'Invalid JSON: ' + e.message;
                errorEl.style.display = '';
                return;
            }

            saveBtn.disabled = true;
            saveBtn.textContent = 'Saving…';

            try {
                const result = await API.createItem(currentConfigName, parsed);
                saveBtn.disabled = false;
                saveBtn.textContent = 'Save';

                if (result.success) {
                    cleanup();
                    // Refresh the table to show the new item
                    loadTableItems(currentConfigName);
                } else {
                    errorEl.textContent = result.error.message || 'Failed to create item.';
                    errorEl.style.display = '';
                }
            } catch (err) {
                saveBtn.disabled = false;
                saveBtn.textContent = 'Save';
                errorEl.textContent = err.message || 'Failed to create item.';
                errorEl.style.display = '';
            }
        }

        saveBtn.addEventListener('click', onSave);
        cancelBtn.addEventListener('click', onCancel);
    }

    /**
     * Render a cell value, handling complex types.
     * @param {*} val
     * @returns {HTMLElement}
     */
    function renderCellValue(val) {
        const span = document.createElement('span');
        span.className = 'cell-value';

        if (val === null || val === undefined) {
            span.textContent = '—';
            span.classList.add('cell-complex');
            return span;
        }

        if (typeof val === 'object') {
            if (Array.isArray(val)) {
                span.textContent = `[${val.length} items]`;
            } else {
                const keys = Object.keys(val);
                span.textContent = `{${keys.length} attrs}`;
            }
            span.classList.add('cell-complex');
            span.title = JSON.stringify(val, null, 2);
            return span;
        }

        if (typeof val === 'boolean') {
            span.textContent = String(val);
            return span;
        }

        span.textContent = String(val);
        return span;
    }

    /**
     * Bulk delete selected items with a confirmation dialog.
     * Shows the confirm modal with item count, then executes DeleteItem
     * for each selected item on confirm.
     * Validates: Requirements 16.1, 16.2, 16.3, 16.4, 16.5
     */
    function bulkDelete() {
        const items = getSelectedRows();
        if (items.length === 0) return;

        const overlay = document.getElementById('confirm-modal');
        const titleEl = document.getElementById('confirm-modal-title');
        const messageEl = document.getElementById('confirm-modal-message');
        const cancelBtn = document.getElementById('confirm-modal-cancel');
        const okBtn = document.getElementById('confirm-modal-ok');

        titleEl.textContent = 'Confirm Delete';
        messageEl.textContent = 'Delete ' + items.length + ' item' + (items.length !== 1 ? 's' : '') + '?';
        overlay.style.display = '';

        function cleanup() {
            overlay.style.display = 'none';
            okBtn.removeEventListener('click', onConfirm);
            cancelBtn.removeEventListener('click', onCancel);
        }

        function onCancel() {
            cleanup();
        }

        async function onConfirm() {
            okBtn.disabled = true;
            okBtn.textContent = 'Deleting…';

            const failures = [];

            for (const item of items) {
                const key = extractItemKey(item);
                try {
                    const result = await API.deleteItem(currentConfigName, key);
                    if (!result.success) {
                        failures.push({ key, error: result.error.message || 'Delete failed' });
                    }
                } catch (err) {
                    failures.push({ key, error: err.message || 'Delete failed' });
                }
            }

            okBtn.disabled = false;
            okBtn.textContent = 'Confirm';
            cleanup();

            if (failures.length === 0) {
                loadTableItems(currentConfigName);
            } else {
                const failedList = failures.map(f => JSON.stringify(f.key) + ': ' + f.error).join('\n');
                App.showGlobalError('Failed to delete ' + failures.length + ' item(s):\n' + failedList);
                // Still refresh to reflect any successful deletions
                loadTableItems(currentConfigName);
            }
        }

        okBtn.addEventListener('click', onConfirm);
        cancelBtn.addEventListener('click', onCancel);
    }

    /**
     * Convert a typed item (with {value, type} attributes) to a plain object
     * (with just the values) for the PutItem API.
     * For untyped items, returns the item as-is.
     * @param {object} item
     * @returns {object} untyped item
     */
    function convertTypedItemToUntyped(item) {
        const result = {};
        for (const key of Object.keys(item)) {
            const val = item[key];
            if (isTypedValue(val)) {
                result[key] = val.value;
            } else {
                result[key] = val;
            }
        }
        return result;
    }

    /**
     * Bulk duplicate selected items with modified partition key.
     * For String partition keys, appends "-copy" suffix.
     * For Number partition keys, increments by 1.
     * Validates: Requirements 17.1, 17.2, 17.3, 17.4, 17.5
     */
    async function bulkDuplicate() {
        const items = getSelectedRows();
        if (items.length === 0) return;

        // Determine partition key name and type from metadata
        let pkName = null;
        let pkAttrType = null;
        if (currentMetadata && currentMetadata.key_schema) {
            const hashKey = currentMetadata.key_schema.find(k => k.key_type === 'HASH');
            if (hashKey) {
                pkName = hashKey.attribute_name;
                pkAttrType = getAttributeType(pkName);
            }
        }

        const failures = [];
        const successes = [];

        for (const item of items) {
            try {
                // Deep copy the item
                const copy = JSON.parse(JSON.stringify(item));

                // Modify the partition key value
                if (pkName && copy[pkName] !== undefined) {
                    if (isTypedValue(copy[pkName])) {
                        // Typed item: check the type field
                        if (copy[pkName].type === 'S') {
                            copy[pkName].value = copy[pkName].value + '-copy';
                        } else if (copy[pkName].type === 'N') {
                            copy[pkName].value = String(Number(copy[pkName].value) + 1);
                        }
                    } else {
                        // Untyped item: check the JS type
                        if (typeof copy[pkName] === 'string') {
                            copy[pkName] = copy[pkName] + '-copy';
                        } else if (typeof copy[pkName] === 'number') {
                            copy[pkName] = copy[pkName] + 1;
                        }
                    }
                }

                // Convert to untyped format for the PutItem API
                const untypedItem = convertTypedItemToUntyped(copy);

                const result = await API.createItem(currentConfigName, untypedItem);
                if (result.success) {
                    successes.push(untypedItem);
                } else {
                    const keyDesc = pkName ? String(untypedItem[pkName]) : JSON.stringify(extractItemKey(item));
                    failures.push({ key: keyDesc, error: result.error.message || 'Duplicate failed' });
                }
            } catch (err) {
                const keyDesc = pkName && item[pkName] !== undefined
                    ? String(getCellDisplayValue(item[pkName]))
                    : JSON.stringify(extractItemKey(item));
                failures.push({ key: keyDesc, error: err.message || 'Duplicate failed' });
            }
        }

        if (failures.length > 0) {
            const failedList = failures.map(f => f.key + ': ' + f.error).join('\n');
            App.showGlobalError('Failed to duplicate ' + failures.length + ' item(s):\n' + failedList);
        }

        // Refresh table to show new items (whether all succeeded or some failed)
        loadTableItems(currentConfigName);
    }

    /**
     * Update the bulk action bar visibility and content based on the
     * current selection state. Shows the bar when one or more rows are
     * selected; hides it otherwise.
     * Validates: Requirements 15.4, 16.1, 17.1
     */
    function updateBulkActionBar() {
        const bar = document.getElementById('bulk-action-bar');
        if (!bar) return;

        if (selectedRows.size > 0) {
            bar.style.display = '';
            const countSpan = bar.querySelector('.bulk-count');
            if (countSpan) {
                countSpan.textContent = selectedRows.size + ' selected';
            }
        } else {
            bar.style.display = 'none';
        }
    }

    /**
     * Get the currently selected items (from checkbox selection).
     * @returns {Array} array of selected item objects
     */
    function getSelectedRows() {
        return Array.from(selectedRows)
            .filter(idx => idx >= 0 && idx < currentItems.length)
            .map(idx => currentItems[idx]);
    }

    /**
     * Refresh the current table view.
     */
    function refresh() {
        if (currentConfigName) {
            loadTableItems(currentConfigName);
        }
    }

    /**
     * Get the current config name being browsed.
     * @returns {string|null}
     */
    function getCurrentConfigName() {
        return currentConfigName;
    }

    /**
     * Get current items.
     * @returns {Array}
     */
    function getItems() {
        return currentItems;
    }

    /**
     * Get current table metadata.
     * @returns {object|null}
     */
    function getMetadata() {
        return currentMetadata;
    }

    // Initialize event listeners on module load
    initDataSelectionPanel();

    return {
        loadTableItems,
        renderTableBrowser,
        refresh,
        getCurrentConfigName,
        getItems,
        getMetadata,
        getLastResponse,
        getSelectedRows,
        validateQueryConstraints,
        buildFilterExpression,
        buildProjectionExpression,
        executeOperation,
        loadMoreItems,
    };
})();
