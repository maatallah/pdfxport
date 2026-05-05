(function () {
    const origCreate = URL.createObjectURL;

    URL.createObjectURL = function (blob) {
        try {
            if (blob && blob.type === "application/pdf" && blob.size > 1000) {
                console.log("📥 BLOB PDF captured:", blob.size);

                window.postMessage({
                    type: "PDF_BLOB",
                    blob
                });
            }
        } catch (e) { }

        return origCreate.apply(this, arguments);
    };
})();