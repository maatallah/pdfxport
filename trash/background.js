
// ======================================================
// Background service worker
// ======================================================
console.log("🟢 Background service worker loaded");


// ======================================================
// Message listener
// ======================================================
chrome.runtime.onMessage.addListener((msg, sender, sendResponse) => {

    if (msg.type !== "UPLOAD_PDF") return;

    console.log("📩 Background received PDF");
    console.log("📦 Size:", msg.size);

    try {
        // Decode base64 → binary
        const binary = atob(msg.base64);

        const bytes = new Uint8Array(binary.length);

        for (let i = 0; i < binary.length; i++) {
            bytes[i] = binary.charCodeAt(i);
        }

        const blob = new Blob([bytes], {
            type: msg.mime || "application/pdf"
        });

        console.log("🚀 Uploading to Go server...");

        fetch("http://localhost:8765/upload", {
            method: "POST",
            body: blob
        })
            .then(() => {
                console.log("✅ Upload success");
                sendResponse({ ok: true });
            })
            .catch(err => {
                console.error("❌ Upload failed:", err);
                sendResponse({ ok: false, error: err.message });
            });

    } catch (err) {
        console.error("❌ Decode error:", err);
        sendResponse({ ok: false, error: err.message });
    }

    // REQUIRED for async response
    return true;
});