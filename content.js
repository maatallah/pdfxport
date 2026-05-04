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

    // Helper to send logs quietly to the Go server (uses sendBeacon for reliability)
    function remoteLog(message) {
        const timestamp = new Date().toISOString().replace('T', ' ').substring(0, 19);
        const fullMsg = `[${timestamp}] ${message}`;
        navigator.sendBeacon('http://localhost:8765/log', fullMsg);
    }

    remoteLog('[INIT] CPT Decoloop Interceptor (MAIN WORLD) loaded on ' + window.location.href);

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

    // --- XHR HOOK (REWRITTEN FOR STABILITY) ---
    const oldOpen = XMLHttpRequest.prototype.open;
    const oldSend = XMLHttpRequest.prototype.send;

    XMLHttpRequest.prototype.open = function(method, url) {
        this._method = method;
        this._url = url;
        return oldOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.send = function() {
        const args = arguments;
        const url = this._url || "UNKNOWN";
        const method = this._method || "POST";

        // Log EVERY send so we can see the URLs clearly
        remoteLog(`[SENDING] ${method} ${url}`);

        if (url.toLowerCase().includes('generatepdfdocument')) {
            remoteLog(`[DEBUG] TARGET DETECTED. Forcing responseType='blob' for: ${url}`);
            try {
                this.responseType = 'blob';
            } catch (e) {
                remoteLog(`[ERROR] Could not set responseType: ${e.message}`);
            }
        }

        this.addEventListener('load', function() {
            try {
                const ct = this.getResponseHeader('Content-Type') || "";
                const status = this.status;
                const rType = this.responseType;
                
                remoteLog(`[XHR LOAD] ${method} ${url} - Status: ${status} CT: ${ct} Type: ${rType}`);

                if (ct.toLowerCase().includes('application/pdf') || url.toLowerCase().includes('generatepdfdocument')) {
                    remoteLog(`🚀 PDF Detected! Size: ${this.response ? (this.response.size || 'N/A') : 'NULL'} bytes`);
                    if (this.response instanceof Blob) {
                        sendToProcessor(this.response);
                    } else {
                        remoteLog(`[WARNING] Response is NOT a Blob. It is: ${typeof this.response}`);
                    }
                }
            } catch (err) {
                remoteLog(`[CRASH] XHR Load: ${err.message}`);
            }
        });

        return oldSend.apply(this, args);
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
