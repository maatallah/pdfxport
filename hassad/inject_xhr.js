/**
 * inject_xhr.js
 * 
 * CORE INTERCEPTOR: This script is injected directly into the page context.
 * It monkey-patches the native XMLHttpRequest object to intercept Decoloop API traffic.
 */

(function () {
    const origOpen = XMLHttpRequest.prototype.open;
    const origSend = XMLHttpRequest.prototype.send;
    const origSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;
    
    // Flag to track if we should be doing anything at all
    let isHarvestingEnabled = true;

    // Listen for state changes from the extension bridge
    window.addEventListener("message", (event) => {
        if (event.source !== window) return;
        if (event.data.type === "SET_HARVEST_STATE") {
            isHarvestingEnabled = !!event.data.enabled;
        }
    });
    
    // Global store for the latest Bearer token captured from live traffic.
    // This allows the Go server to remain authenticated without manual intervention.
    let authHeader = null;

    /**
     * Hook into .open() to capture the request URL.
     */
    XMLHttpRequest.prototype.open = function (method, url) {
        this._url = url;
        return origOpen.apply(this, arguments);
    };

    /**
     * Hook into .setRequestHeader() to capture the Bearer token.
     * Decoloop sends this in almost every background request.
     */
    XMLHttpRequest.prototype.setRequestHeader = function(header, value) {
        if (header.toLowerCase() === 'authorization') {
            authHeader = value; 
        }
        return origSetRequestHeader.apply(this, arguments);
    };

    /**
     * Hook into .send() to listen for the response.
     */
    XMLHttpRequest.prototype.send = function (body) {
        this.addEventListener("load", function () {
            // NEW: Ignore everything if harvesting is disabled
            if (!isHarvestingEnabled) return;

            // Only proceed if we have a URL and a response
            if (!this._url || !this.responseText) return;

            try {
                const data = JSON.parse(this.responseText);
                
                /**
                 * harvest() processes a single project object.
                 * If polygons are missing (common in dashboard summaries), it triggers
                 * a background fetch to get the full project details.
                 */
                const harvest = (item) => {
                    const order = item.projectNumber || item.prjNumber;
                    const id = item.id;
                    let polygons = [];
                    
                    // Support multiple naming conventions found in Sieval API
                    if (item.polygons && Array.isArray(item.polygons)) {
                        polygons = item.polygons.map(p => p.id);
                    } else if (item.polygonProjectMaterials && Array.isArray(item.polygonProjectMaterials)) {
                        polygons = item.polygonProjectMaterials.map(p => p.id);
                    }
                    
                    if (order && id) {
                        // CASE A: We have IDs but no Polygons (Dashboard Summary)
                        if (polygons.length === 0 && authHeader) {
                            // We trigger a native 'fetch' which is NOT intercepted by our XHR hook
                            // to avoid infinite recursion.
                            fetch(`https://sievalhub.sieval.com/api/productiondata/project/getProjectDetailsById?productionProjectId=${id}`, {
                                headers: { 'Authorization': authHeader }
                            })
                            .then(r => r.json())
                            .then(fullItem => {
                                if (fullItem && (fullItem.polygons || fullItem.polygonProjectMaterials)) {
                                    harvest(fullItem); // Re-process with full data
                                }
                            }).catch(() => {});
                            return;
                        }

                        // CASE B: We have full data (IDs + Polygons)
                        if (polygons.length > 0) {
                            // Forward the complete bundle to the Content Script (content_main.js)
                            window.postMessage({
                                type: "HARVESTED_ID",
                                orderNum: order,
                                projectId: id,
                                polygons: polygons,
                                token: authHeader ? authHeader.replace("Bearer ", "") : ""
                            }, "*");
                        }
                    }
                };

                // Filter logic to ensure we only process relevant API responses
                if (this._url.includes("browse")) {
                    if (data.projects && Array.isArray(data.projects)) {
                        data.projects.forEach((p) => harvest(p));
                    }
                } else if (this._url.includes("getProjectDetailsById") && !this._url.includes("X-PDFXport")) {
                    harvest(data);
                }
            } catch (e) {
                // Silently ignore non-JSON or unrelated traffic
            }
        });

        return origSend.apply(this, arguments);
    };
})();