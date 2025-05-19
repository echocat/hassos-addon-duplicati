(() => {
    const originalFetch = window.fetch;
    window.fetch = function (input, init) {
        if (typeof input === 'string' && input.startsWith('/api/')) {
            input = __wrapperPrefix__ + input;
        }
        return originalFetch(input, init);
    };

    const originalOpen = XMLHttpRequest.prototype.open;
    XMLHttpRequest.prototype.open = function (method, url, ...args) {
        if (url.startsWith('/api/')) {
            url = __wrapperPrefix__ + url;
        }
        return originalOpen.call(this, method, url, ...args);
    };

    const originalWebSocket = window.WebSocket;

    class InterceptingWebSocket extends originalWebSocket {

        static patchPath(path) {
            try {
                const absolutePath = (() => {
                    if (path.startsWith("/")) {
                        return path;
                    }
                    const base = new URL(document.baseURI || window.location.href);
                    return new URL(path, base).pathname;
                })()
                return __wrapperPrefix__.replace(/\/+$/, "") + "/" + absolutePath.replace(/^\/+/, "");
            } catch (e) {
                return path;
            }
        }

        constructor(url, protocols) {
            if (typeof url === 'string') {
                try {
                    url = new URL(url);
                } catch (ignored) {
                }
            }
            if (typeof url === 'string') {
                url = InterceptingWebSocket.patchPath(url);
            } else {
                url.pathname = InterceptingWebSocket.patchPath(url.pathname);
            }
            super(url, protocols);
        }

    }

    window.WebSocket = InterceptingWebSocket;
})();

