(function() {
    console.log(">> CPT DECOLOOP INTERCEPTOR v1.7 ACTIVE <<");
    const _nativeFetch = window.fetch.bind(window);

    // Helper to send PDF to the Go server (uses native fetch, bypasses our hook)
    async function sendToProcessor(blob) {
        try {
            const res = await _nativeFetch('http://localhost:8765/upload', {
                method: 'POST',
                body: blob
            });
            if (res.ok) {
                remoteLog("[OK] PDF pushed to Go Server successfully!");
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

    remoteLog('[INIT] v1.7 (Total Suppression) loaded on ' + window.location.href);

    // --- GREEDY PDF EXTRACTION & SUPPRESSION ---
    let lastInterceptTime = 0;

    function isRecentlyIntercepted() {
        return (Date.now() - lastInterceptTime < 3000);
    }

    // 1. Block window.open
    const oldWindowOpen = window.open;
    window.open = function(url) {
        if (isRecentlyIntercepted()) {
            remoteLog("[OK] Suppressing window.open() print preview");
            return null;
        }
        return oldWindowOpen.apply(this, arguments);
    };

    // 2. Block window.print (The final piece!)
    const oldWindowPrint = window.print;
    window.print = function() {
        if (isRecentlyIntercepted()) {
            remoteLog("[OK] Suppressing window.print() dialog");
            return;
        }
        return oldWindowPrint.apply(this, arguments);
    };

    // 3. Block iFrame PDF loading (Common for print previews)
    const iframeSrcDescriptor = Object.getOwnPropertyDescriptor(HTMLIFrameElement.prototype, 'src');
    Object.defineProperty(HTMLIFrameElement.prototype, 'src', {
        set: function(val) {
            if (isRecentlyIntercepted() && (val.includes('blob:') || val.includes('.pdf'))) {
                remoteLog("[OK] Suppressing iFrame PDF preview source");
                return;
            }
            if (iframeSrcDescriptor && iframeSrcDescriptor.set) {
                iframeSrcDescriptor.set.call(this, val);
            }
        }
    });

    // 3. Block Hidden Link Clicks
    const oldAnchorClick = HTMLAnchorElement.prototype.click;
    HTMLAnchorElement.prototype.click = function() {
        if (isRecentlyIntercepted() && (this.href.includes('blob:') || this.href.includes('.pdf'))) {
            remoteLog("[OK] Suppressing Hidden Link PDF download/preview");
            return;
        }
        return oldAnchorClick.apply(this, arguments);
    };

    // --- XHR HOOK ---
    const oldOpen = XMLHttpRequest.prototype.open;
    const oldSend = XMLHttpRequest.prototype.send;

    XMLHttpRequest.prototype.open = function(method, url) {
        this._method = method;
        this._url = url;
        return oldOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.send = function() {
        const url = this._url || "";
        if (url.toLowerCase().includes('generatepdfdocument')) {
            this.responseType = 'blob';
        }

        this.addEventListener('load', function() {
            try {
                const ct = this.getResponseHeader('Content-Type') || "";
                if (ct.toLowerCase().includes('application/pdf') || url.toLowerCase().includes('generatepdfdocument')) {
                    if (this.response instanceof Blob && this.response.size > 0) {
                        lastInterceptTime = Date.now();
                        remoteLog(`🚀 PDF Captured (${this.response.size} bytes). Suppressing UI...`);
                        sendToProcessor(this.response);
                    }
                }
            } catch (err) {
                remoteLog(`[CRASH] XHR Load: ${err.message}`);
            }
        });
        return oldSend.apply(this, arguments);
    };

    // --- FETCH HOOK ---
    const oldFetch = window.fetch;
    window.fetch = async function(...args) {
        let reqUrl = (typeof args[0] === 'string') ? args[0] : (args[0] && args[0].url ? args[0].url : "");
        try {
            const response = await oldFetch.apply(this, args);
            const ct = response.headers.get('Content-Type') || "";
            if (ct.toLowerCase().includes('application/pdf') || reqUrl.toLowerCase().includes('generatepdfdocument')) {
                // Greedy: Consume body without cloning
                try {
                    const blob = await response.blob();
                    if (blob && blob.size > 0) {
                        lastInterceptTime = Date.now();
                        remoteLog(`🚀 PDF Captured via FETCH. Suppressing UI...`);
                        sendToProcessor(blob);
                    }
                } catch (e) {}
            }
            return response;
        } catch (error) {
            throw error;
        }
    };
})();
