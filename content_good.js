console.log("Decoloop Auto-Downloader Active!");

const originalOpen = XMLHttpRequest.prototype.open;
const originalSend = XMLHttpRequest.prototype.send;

XMLHttpRequest.prototype.open = function(method, url) {
    this._url = url;
    return originalOpen.apply(this, arguments);
};

XMLHttpRequest.prototype.send = function(body) {
    this.addEventListener('load', function() {
        if (this._url && this._url.includes('generatePdfDocument')) {
            console.log("🚀 PDF Intercepted via XHR!");
            
            let blob = null;
            if (this.responseType === 'blob') {
                blob = this.response;
            } else if (this.responseType === 'arraybuffer') {
                blob = new Blob([this.response], {type: 'application/pdf'});
            }

            if (blob) {
                const url = URL.createObjectURL(blob);
                const a = document.createElement('a');
                a.href = url;
                
                const d = new Date();
                const timeStr = d.getHours() + "h" + d.getMinutes() + "m";
                a.download = 'Decoloop_Commandes_' + timeStr + '.pdf';
                
                document.body.appendChild(a);
                a.click();
                document.body.removeChild(a);
                URL.revokeObjectURL(url);
            }
        }
    });
    
    return originalSend.apply(this, arguments);
};
