(function() {
    console.log(">> CPT DECOLOOP INTERCEPTOR ACTIVE <<");

    // Helper to send PDF to the Go server
    async function sendToProcessor(blob) {
        try {
            console.log("[Extension] Pushing PDF to local server...");
            await fetch('http://localhost:8765/upload', {
                method: 'POST',
                body: blob
            });
            console.log("[OK] PDF pushed to Go Server successfully!");
        } catch (err) {
            console.error("[ERROR] Failed to talk to Go Server. Is it running on port 8765?", err);
        }
    }

    // Helper to send logs quietly to the Go server
    function remoteLog(message) {
        fetch('http://localhost:8765/log', {
            method: 'POST',
            body: message
        }).catch(e => {}); // Ignore errors so we don't spam the console if server is down
    }

    // --- GLOBAL ERROR CATCHERS ---
    window.addEventListener('error', function(e) {
        remoteLog(`[ERROR] ${e.message} at ${e.filename}:${e.lineno}:${e.colno}`);
    });

    window.addEventListener('unhandledrejection', function(e) {
        remoteLog(`[PROMISE REJECTION] ${e.reason}`);
    });

    // --- USER ACTION TRACING ---
    document.addEventListener('click', function(e) {
        let target = e.target;
        if (target && target.tagName) {
            let text = target.innerText ? target.innerText.substring(0, 50).trim() : '';
            let tag = target.tagName;
            let id = target.id ? `#${target.id}` : '';
            let cls = target.className ? `.${target.className}` : '';
            remoteLog(`[CLICK] ${tag}${id}${cls} - Text: "${text}"`);
        }
    });

    // 1. Hook XMLHttpRequest (Legacy)
    const oldOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function(method, url) {
        this.addEventListener('load', function() {
            const ct = this.getResponseHeader('Content-Type');
            const status = this.status;
            
            // Log the network request
            remoteLog(`[XHR] ${method} ${url} - Status: ${status} CT: ${ct}`);

            // GUESS: If Content-Type is PDF, we grab it regardless of URL
            // FALLBACK: If URL contains generatePdfDocument, grab it even if Content-Type is wrong
            const resUrl = this.responseURL || url || "";
            if ((ct && ct.includes('application/pdf')) || resUrl.includes('generatePdfDocument')) {
                console.log(`🚀 PDF Detected via XHR (URL: ${resUrl}, CT: ${ct})`);
                
                let blob = null;
                if (this.responseType === 'blob') {
                    blob = this.response;
                } else if (this.responseType === 'arraybuffer') {
                    blob = new Blob([this.response], {type: 'application/pdf'});
                } else {
                    // Fallback if responseType wasn't set correctly
                    blob = new Blob([this.response], {type: 'application/pdf'});
                }
                
                if (blob) {
                    sendToProcessor(blob);
                }
            }
        });
        return oldOpen.apply(this, arguments);
    };

    // 2. Hook Fetch (Modern)
    const oldFetch = window.fetch;
    window.fetch = async function(...args) {
        let reqMethod = "GET";
        let reqUrl = "";
        
        if (typeof args[0] === 'string') {
            reqUrl = args[0];
            if (args[1] && args[1].method) reqMethod = args[1].method;
        } else if (args[0] && args[0].url) {
            reqUrl = args[0].url;
            reqMethod = args[0].method || "GET";
        }

        try {
            const response = await oldFetch.apply(this, args);
            const ct = response.headers.get('Content-Type');
            const status = response.status;
            
            // Log the network request
            remoteLog(`[FETCH] ${reqMethod} ${reqUrl} - Status: ${status} CT: ${ct}`);
            
            // GUESS: If Content-Type is PDF, we grab it regardless of URL
            // FALLBACK: If URL contains generatePdfDocument, grab it even if Content-Type is wrong
            const resUrl = response.url || reqUrl || "";
            if ((ct && ct.includes('application/pdf')) || resUrl.includes('generatePdfDocument')) {
                console.log(`🚀 PDF Detected via FETCH (URL: ${resUrl}, CT: ${ct})`);
                
                // Clone the response so the website can still use it
                const clone = response.clone();
                const blob = await clone.blob();
                
                if (blob) {
                    sendToProcessor(blob);
                }
            }
            return response;
        } catch (error) {
            remoteLog(`[FETCH ERROR] ${reqMethod} ${reqUrl} - ${error.message}`);
            throw error;
        }
    };
})();
