(function () {
    const origOpen = XMLHttpRequest.prototype.open;
    const origSend = XMLHttpRequest.prototype.send;

    XMLHttpRequest.prototype.open = function (method, url) {
        this._url = url;
        return origOpen.apply(this, arguments);
    };

    XMLHttpRequest.prototype.send = function (body) {
        if (this._url && this._url.includes("generatePdfDocument")) {
            this.responseType = "blob";
        }

        this.addEventListener("load", function () {
            if (!this._url) return;

            if (this._url.includes("generatePdfDocument")) {
                const blob = this.response;

                if (blob && blob.size > 1000) {
                    console.log("📥 XHR PDF captured:", blob.size);

                    window.postMessage({
                        type: "PDF_BLOB",
                        blob
                    });
                }
            }
        });

        return origSend.apply(this, arguments);
    };
})();