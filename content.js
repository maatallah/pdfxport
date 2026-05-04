(function() {
    console.log(">> CPT DECOLOOP INTERCEPTOR ACTIVE <<");
    // Save the NATIVE fetch before we hook anything (prevents recursive loops)
    const _nativeFetch = window.fetch.bind(window);

    // Helper to send PDF to the Go server (uses native fetch, bypasses our hook)
    async function sendToProcessor(blob) {
        try {
            remoteLog(`[DEBUG] Attempting to push PDF to Go server (${blob.size} bytes)...`);
            const res = await _nativeFetch('http://localhost:8765/upload', {
                method: 'POST',
                body: blob
            });
            if (res.ok) {
                remoteLog("[OK] PDF pushed to Go Server successfully!");
            } else {
                remoteLog(`[ERROR] Go Server returned status: ${res.status}`);
            }
        } catch (err) {
            remoteLog(`[ERROR] Failed to talk to Go Server: ${err.message}`);
        }
    }

    // Helper to send logs quietly to the Go server (uses native fetch, bypasses our hook)
    function remoteLog(message) {
        _nativeFetch('http://localhost:8765/log', {
            method: 'POST',
            body: message
        }).catch(e => {}); // Ignore errors so we don't spam the console if server is down
    }

    remoteLog('[INIT] CPT Decoloop Interceptor loaded on ' + window.location.href);

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
            try {
                const ct = this.getResponseHeader('Content-Type') || "";
                const status = this.status;
                const resUrl = this.responseURL || url || "";
                
                // Log the network request
                remoteLog(`[XHR] ${method} ${url} - Status: ${status} CT: ${ct}`);

                // Breadcrumb 1: Check conditions
                const isPdfCT = ct.toLowerCase().includes('application/pdf');
                const isPdfUrl = resUrl.toLowerCase().includes('generatepdfdocument');

                if (isPdfCT || isPdfUrl) {
                    remoteLog(`[DEBUG] Step 1: PDF Condition met (CT: ${isPdfCT}, URL: ${isPdfUrl})`);
                    remoteLog(`🚀 PDF Detected via XHR! (URL: ${resUrl})`);
                    
                    // Breadcrumb 2: Check response type/data
                    remoteLog(`[DEBUG] Step 2: Checking response data...`);
                    let blob = null;
                    if (this.response instanceof Blob) {
                        remoteLog(`[DEBUG] Step 3: Response is already a Blob (${this.response.size} bytes)`);
                        blob = this.response;
                    } else if (this.response instanceof ArrayBuffer) {
                        remoteLog(`[DEBUG] Step 3: Response is ArrayBuffer, converting...`);
                        blob = new Blob([this.response], {type: 'application/pdf'});
                    } else {
                        remoteLog(`[DEBUG] Step 3: Response is unknown type (${typeof this.response}), attempting conversion...`);
                        try {
                            blob = new Blob([this.response], {type: 'application/pdf'});
                        } catch (e) {
                            remoteLog(`[ERROR] Failed to create blob: ${e.message}`);
                        }
                    }
                    
                    if (blob && blob.size > 0) {
                        remoteLog(`[DEBUG] Step 4: Final Blob ready (${blob.size} bytes). Sending to processor...`);
                        sendToProcessor(blob);
                    } else {
                        remoteLog("[WARNING] Step 4: Resulting Blob is empty or invalid.");
                    }
                }
            } catch (err) {
                remoteLog(`[CRASH] Error in XHR Load Listener: ${err.message}\nStack: ${err.stack}`);
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
            
            // Skip logging our own requests to localhost to avoid noise
            if (!reqUrl.includes('localhost:8765')) {
                remoteLog(`[FETCH] ${reqMethod} ${reqUrl} - Status: ${status} CT: ${ct}`);
            }
            
            // GUESS: If Content-Type is PDF, we grab it regardless of URL
            // FALLBACK: If URL contains generatePdfDocument, grab it even if Content-Type is wrong
            const resUrl = response.url || reqUrl || "";
            if ((ct && ct.includes('application/pdf')) || resUrl.includes('generatePdfDocument')) {
                remoteLog(`🚀 PDF Detected via FETCH! (URL: ${resUrl}, CT: ${ct})`);
                
                // Clone the response so the website can still use it
                const clone = response.clone();
                try {
                    const blob = await clone.blob();
                    if (blob && blob.size > 0) {
                        sendToProcessor(blob);
                    } else {
                        remoteLog("[WARNING] Captured FETCH response was empty.");
                    }
                } catch (e) {
                    remoteLog(`[ERROR] Failed to extract blob from FETCH: ${e.message}`);
                }
            }
            return response;
        } catch (error) {
            remoteLog(`[FETCH ERROR] ${reqMethod} ${reqUrl} - ${error.message}`);
            throw error;
        }
    };
})();
