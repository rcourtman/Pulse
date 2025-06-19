// Loading skeleton components for better perceived performance
PulseApp.ui = PulseApp.ui || {};

PulseApp.ui.loadingSkeletons = (() => {
    
    function createTableRowSkeleton(columns = 11) {
        const cells = [];
        for (let i = 0; i < columns; i++) {
            cells.push(`
                <td class="p-1 px-2">
                    <div class="h-4 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
            `);
        }
        
        return `
            <tr class="border-b border-gray-200 dark:border-gray-700">
                ${cells.join('')}
            </tr>
        `;
    }
    
    function createTableSkeleton(rows = 5, columns = 11) {
        const skeletonRows = [];
        for (let i = 0; i < rows; i++) {
            skeletonRows.push(createTableRowSkeleton(columns));
        }
        
        return skeletonRows.join('');
    }
    
    function createNodeCardSkeleton() {
        return `
            <div class="bg-white dark:bg-gray-800 rounded-lg p-4 shadow-sm border border-gray-200 dark:border-gray-700">
                <div class="flex items-center justify-between mb-3">
                    <div class="h-6 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                    <div class="h-5 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </div>
                <div class="space-y-2">
                    <div class="flex justify-between items-center">
                        <div class="h-4 w-12 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                        <div class="h-4 w-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                    </div>
                    <div class="h-3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </div>
                <div class="space-y-2 mt-2">
                    <div class="flex justify-between items-center">
                        <div class="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                        <div class="h-4 w-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                    </div>
                    <div class="h-3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </div>
            </div>
        `;
    }
    
    function createStorageRowSkeleton() {
        return `
            <tr class="transition-all duration-150">
                <td class="p-1 px-2">
                    <div class="h-4 w-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-4 w-12 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-3 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-1 px-2">
                    <div class="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
            </tr>
        `;
    }
    
    function createBackupRowSkeleton() {
        return `
            <tr class="hover:bg-gray-50 dark:hover:bg-gray-700/50">
                <td class="p-2 px-3">
                    <div class="h-4 w-32 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-2 px-3">
                    <div class="h-4 w-16 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-2 px-3">
                    <div class="h-4 w-24 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-2 px-3">
                    <div class="h-4 w-20 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-2 px-3">
                    <div class="h-4 w-12 bg-gray-200 dark:bg-gray-700 rounded animate-pulse"></div>
                </td>
                <td class="p-2 px-3">
                    <div class="h-5 w-16 bg-gray-200 dark:bg-gray-700 rounded-full animate-pulse"></div>
                </td>
            </tr>
        `;
    }
    
    function showTableSkeleton(tableElement, rows = 5, columns = 11) {
        if (!tableElement) return;
        
        const tbody = tableElement.querySelector('tbody');
        if (tbody) {
            tbody.innerHTML = createTableSkeleton(rows, columns);
        }
    }
    
    function showNodeCardsSkeleton(container, count = 3) {
        if (!container) return;
        
        const skeletons = [];
        for (let i = 0; i < count; i++) {
            skeletons.push(createNodeCardSkeleton());
        }
        
        container.innerHTML = `
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                ${skeletons.join('')}
            </div>
        `;
    }
    
    return {
        createTableSkeleton,
        showTableSkeleton,
        showNodeCardsSkeleton
    };
})();