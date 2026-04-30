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

    // 1. Hook XMLHttpRequest (Legacy)
    const oldOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function() {
        this.addEventListener('load', function() {
            const ct = this.getResponseHeader('Content-Type');
            // GUESS: If Content-Type is PDF, we grab it regardless of URL
            if (ct && ct.includes('application/pdf')) {
                console.log("🚀 PDF Detected via XHR (Content-Type Match)");
                
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
        const response = await oldFetch.apply(this, args);
        const ct = response.headers.get('Content-Type');
        
        // GUESS: If Content-Type is PDF, we grab it regardless of URL
        if (ct && ct.includes('application/pdf')) {
            console.log("🚀 PDF Detected via FETCH (Content-Type Match)");
            
            // Clone the response so the website can still use it
            const clone = response.clone();
            const blob = await clone.blob();
            
            if (blob) {
                sendToProcessor(blob);
            }
        }
        return response;
    };
})();
