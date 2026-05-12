/**
 * inject_xhr.js
 * 
 * CORE INTERCEPTOR: Injected into Decoloop page context.
 * Captures JSON data and Auth tokens for the staging buffer.
 */

(function () {
    const origOpen = XMLHttpRequest.prototype.open;
    const origSend = XMLHttpRequest.prototype.send;
    const origSetRequestHeader = XMLHttpRequest.prototype.setRequestHeader;
    
    let isHarvestingEnabled = true;
    let authHeader = null;

    // Listen for state changes
    window.addEventListener("message", (event) => {
        if (event.source !== window) return;
        if (event.data.type === "SET_HARVEST_STATE") {
            isHarvestingEnabled = !!event.data.enabled;
        }
    });

    XMLHttpRequest.prototype.open = function (method, url) {
        this._url = url;
        return origOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.setRequestHeader = function(header, value) {
        if (header.toLowerCase() === 'authorization') {
            authHeader = value; 
        }
        return origSetRequestHeader.apply(this, arguments);
    };

    XMLHttpRequest.prototype.send = function (body) {
        this.addEventListener("load", function () {
            if (!isHarvestingEnabled || !this._url || !this.responseText) return;

            try {
                const data = JSON.parse(this.responseText);
                
                const harvest = (item) => {
                    const order = item.projectNumber || item.prjNumber;
                    const id = item.id;
                    let polygons = [];
                    
                    if (item.polygons && Array.isArray(item.polygons)) {
                        polygons = item.polygons.map(p => p.id);
                    } else if (item.polygonProjectMaterials && Array.isArray(item.polygonProjectMaterials)) {
                        polygons = item.polygonProjectMaterials.map(p => p.id);
                    }
                    
                    if (order && id) {
                        // Background fetch if polygons missing
                        if (polygons.length === 0 && authHeader) {
                            fetch(`https://sievalhub.sieval.com/api/productiondata/project/getProjectDetailsById?productionProjectId=${id}`, {
                                headers: { 'Authorization': authHeader }
                            })
                            .then(r => r.json())
                            .then(fullItem => {
                                if (fullItem && (fullItem.polygons || fullItem.polygonProjectMaterials)) {
                                    harvest(fullItem); 
                                }
                            }).catch(() => {});
                            return;
                        }

                        // Send to STAGING (via content_main.js)
                        if (polygons.length > 0) {
                            window.postMessage({
                                type: "HARVESTED_ID",
                                orderNum: order,
                                projectId: id,
                                polygons: polygons,
                                token: authHeader ? authHeader.replace("Bearer ", "") : ""
                            }, "*");
                        } else {
                            console.warn(`⚠️ Skipped ${order}: No polygons and no Auth for background fetch.`);
                        }
                    }
                };

                if (this._url.includes("browse")) {
                    if (data.projects && Array.isArray(data.projects)) {
                        data.projects.forEach((p) => harvest(p));
                    }
                } else if (this._url.includes("getProjectDetailsById") && !this._url.includes("X-PDFXport")) {
                    harvest(data);
                }
            } catch (e) {}
        });

        return origSend.apply(this, arguments);
    };
})();