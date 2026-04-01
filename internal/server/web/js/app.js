// app.js - Main application initialization
// Validates: Requirements 3.1, 4.1, 5.1, 6.1, 7.1, 8.1

const App = (() => {
    /**
     * Initialize the application: set up event listeners and load data.
     */
    function init() {
        // Table list actions
        document.getElementById('refresh-tables-btn')
            .addEventListener('click', () => Tables.loadTables());

        // Browser actions
        document.getElementById('create-item-btn')
            .addEventListener('click', () => Editor.showCreateForm());

        document.getElementById('refresh-items-btn')
            .addEventListener('click', () => Browser.refresh());

        // Editor actions
        document.getElementById('editor-close-btn')
            .addEventListener('click', () => Editor.close());

        document.getElementById('edit-item-btn')
            .addEventListener('click', () => Editor.showEditForm());

        document.getElementById('delete-item-btn')
            .addEventListener('click', () => Editor.deleteItem());

        document.getElementById('save-item-btn')
            .addEventListener('click', () => Editor.saveItem());

        document.getElementById('cancel-edit-btn')
            .addEventListener('click', () => {
                // If we were editing an existing item, go back to detail view
                // Otherwise close the panel
                const configName = Tables.getSelectedConfigName();
                if (configName) {
                    Editor.close();
                } else {
                    Editor.close();
                }
            });

        // Global error dismiss
        document.getElementById('global-error-close')
            .addEventListener('click', () => hideGlobalError());

        // Load tables on startup
        Tables.loadTables();
    }

    /**
     * Display a global error banner.
     * @param {string} message
     */
    function showGlobalError(message) {
        const el = document.getElementById('global-error');
        document.getElementById('global-error-message').textContent = message;
        el.style.display = '';
    }

    /**
     * Hide the global error banner.
     */
    function hideGlobalError() {
        document.getElementById('global-error').style.display = 'none';
    }

    return { init, showGlobalError, hideGlobalError };
})();

// Start the app when the DOM is ready
document.addEventListener('DOMContentLoaded', App.init);
