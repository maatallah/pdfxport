(function () {
    const origFetch = window.fetch;

    window.fetch = async function (...args) {
        const res = await origFetch.apply(this, args);

        try {
            const url = args[0];

            if (typeof url === "string" && url.includes("generatePdfDocument")) {
                const clone = res.clone();
                const blob = await clone.blob();

                if (blob.size > 1000) {
                    console.log("📥 FETCH PDF captured:", blob.size);

                    window.postMessage({
                        type: "PDF_BLOB",
                        blob
                    });
                }
            }
        } catch (e) {
            console.warn("Fetch intercept error", e);
        }

        return res;
    };
})();