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

    // --- GREEDY PDF EXTRACTION ---
    let lastInterceptTime = 0;

    // Hook createObjectURL to catch when the site tries to preview a blob
    const oldCreateObjectURL = URL.createObjectURL;
    URL.createObjectURL = function(obj) {
        const url = oldCreateObjectURL.apply(this, arguments);
        if (obj instanceof Blob && obj.type === 'application/pdf') {
            remoteLog(`[DEBUG] Website created PDF Blob URL: ${url}`);
        }
        return url;
    };

    // Hook window.open to suppress the preview window
    const oldWindowOpen = window.open;
    window.open = function(url) {
        const now = Date.now();
        // If we recently intercepted a PDF, block window.open for 2 seconds
        if (now - lastInterceptTime < 2000) {
            remoteLog("[OK] Suppressing Print Preview window.open()");
            return null; 
        }
        return oldWindowOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.send = function() {
        const args = arguments;
        const url = this._url || "UNKNOWN";
        const method = this._method || "POST";

        if (url.toLowerCase().includes('generatepdfdocument')) {
            remoteLog(`[DEBUG] Forcing responseType='blob' for: ${url}`);
            this.responseType = 'blob';
        }

        this.addEventListener('load', function() {
            try {
                const ct = this.getResponseHeader('Content-Type') || "";
                if (ct.toLowerCase().includes('application/pdf') || url.toLowerCase().includes('generatepdfdocument')) {
                    const size = this.response ? (this.response.size || 0) : 0;
                    remoteLog(`🚀 PDF Detected! Size: ${size} bytes`);
                    if (this.response instanceof Blob && size > 0) {
                        lastInterceptTime = Date.now(); // Mark time to block window.open
                        sendToProcessor(this.response);
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
        let reqUrl = (typeof args[0] === 'string') ? args[0] : (args[0] && args[0].url ? args[0].url : "");
        
        try {
            const response = await oldFetch.apply(this, args);
            const ct = response.headers.get('Content-Type') || "";
            
            if (ct.toLowerCase().includes('application/pdf') || reqUrl.toLowerCase().includes('generatepdfdocument')) {
                remoteLog(`🚀 PDF Detected via FETCH! (URL: ${reqUrl})`);
                
                // NO CLONE: We consume the body so the website can't use it for a preview
                try {
                    const blob = await response.blob();
                    if (blob && blob.size > 0) {
                        lastInterceptTime = Date.now();
                        sendToProcessor(blob);
                    }
                } catch (e) {
                    remoteLog(`[ERROR] Fetch body already consumed or failed: ${e.message}`);
                }
            }
            return response;
        } catch (error) {
            if (!reqUrl.includes('localhost:8765')) {
                remoteLog(`[FETCH ERROR] ${reqUrl} - ${error.message}`);
            }
            throw error;
        }
    };
})();
