// export.js - CSV and JSON export controller
// Validates: Requirements 18.3, 18.4, 18.5

const Export = (() => {
    /**
     * Generate a timestamp string suitable for filenames.
     * Format: YYYY-MM-DDTHHMMSS (colons and dots replaced with dashes).
     * @returns {string}
     */
    function timestamp() {
        return new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    }

    /**
     * CSV-escape a single value.
     * Wraps in double quotes if the value contains commas, quotes, or newlines.
     * @param {*} val
     * @returns {string}
     */
    function csvEscape(val) {
        if (val === null || val === undefined) return '';
        const str = String(val);
        if (str.includes(',') || str.includes('"') || str.includes('\n')) {
            return '"' + str.replace(/"/g, '""') + '"';
        }
        return str;
    }

    /**
     * Extract a display value from a potentially typed item cell.
     * Typed items use the format { value: <val>, type: "<code>" }.
     * For complex types (M, L), returns a JSON string representation.
     * @param {*} cellVal
     * @returns {*}
     */
    function extractDisplayValue(cellVal) {
        if (cellVal && typeof cellVal === 'object' && 'value' in cellVal && 'type' in cellVal) {
            const t = cellVal.type;
            const v = cellVal.value;
            if (t === 'M' || t === 'L') {
                return JSON.stringify(v);
            }
            if (t === 'NULL') return '';
            return v;
        }
        return cellVal;
    }

    /**
     * Trigger a browser file download with the given content.
     * @param {string} content - file content
     * @param {string} filename - suggested filename
     * @param {string} mimeType - MIME type for the blob
     */
    function triggerDownload(content, filename, mimeType) {
        const blob = new Blob([content], { type: mimeType });
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        a.click();
        URL.revokeObjectURL(url);
    }

    /**
     * Download the current result set as a CSV file.
     * Generates CSV with headers from the provided columns array.
     * @param {Array<object>} items - array of typed item objects
     * @param {Array<string>} columns - ordered column names for headers
     * @param {string} tableName - used in the filename
     */
    function downloadCSV(items, columns, tableName) {
        const header = columns.map(c => csvEscape(c)).join(',');
        const rows = items.map(item =>
            columns.map(col => csvEscape(extractDisplayValue(item[col]))).join(',')
        );
        const csv = [header, ...rows].join('\n');
        const filename = tableName + '_' + timestamp() + '.csv';
        triggerDownload(csv, filename, 'text/csv');
    }

    /**
     * Download the current result set as a JSON file.
     * Converts typed items to plain objects for readability.
     * @param {Array<object>} items - array of typed item objects
     * @param {string} tableName - used in the filename
     */
    function downloadJSON(items, tableName) {
        const json = JSON.stringify(items, null, 2);
        const filename = tableName + '_' + timestamp() + '.json';
        triggerDownload(json, filename, 'application/json');
    }

    return { downloadCSV, downloadJSON };
})();
