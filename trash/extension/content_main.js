// ======================================================
// Inject page-level interception scripts
// ======================================================
function inject(file) {
    const s = document.createElement("script");
    s.src = chrome.runtime.getURL(file);

    s.onload = () => {
        console.log("✅ Injected:", file);
        s.remove();
    };

    s.onerror = (e) => {
        console.error("❌ Injection failed:", file, e);
    };

    (document.head || document.documentElement).appendChild(s);
}

// Order matters
inject("inject_xhr.js");
inject("inject_fetch.js");
inject("inject_blob.js");

console.log("🚀 Interception layers injected");


// ======================================================
// Deduplication
// ======================================================
const seen = new Set();

function hashBlob(blob) {
    return blob.size + "_" + blob.type;
}


// ======================================================
// Safe Base64 conversion (MV3-safe transport)
// ======================================================
function arrayBufferToBase64(buffer) {
    let binary = "";
    const bytes = new Uint8Array(buffer);

    for (let i = 0; i < bytes.length; i++) {
        binary += String.fromCharCode(bytes[i]);
    }

    return btoa(binary);
}


// ======================================================
// Send PDF to background safely
// ======================================================
async function sendBlob(blob) {
    try {
        const buffer = await blob.arrayBuffer();
        const base64 = arrayBufferToBase64(buffer);

        console.log("📤 Sending PDF:", blob.size);

        chrome.runtime.sendMessage({
            type: "UPLOAD_PDF",
            base64: base64,
            mime: blob.type,
            size: blob.size
        });

    } catch (err) {
        console.error("❌ Failed to encode blob:", err);
    }
}


// ======================================================
// Single message listener
// ======================================================
window.addEventListener("message", (event) => {
    if (event.source !== window) return;
    if (!event.data || event.data.type !== "PDF_BLOB") return;

    const blob = event.data.blob;

    if (!blob) return;

    const key = hashBlob(blob);

    if (seen.has(key)) {
        console.log("⚠️ Duplicate skipped:", key);
        return;
    }

    seen.add(key);

    console.log("📥 PDF captured:", blob.size);

    sendBlob(blob);
});