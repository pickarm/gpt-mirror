FROM mcr.microsoft.com/playwright:v1.62.1-noble

WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends gosu && \
    rm -rf /var/lib/apt/lists/*

COPY browser-worker/package.json ./package.json
RUN PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm install --omit=dev --no-audit --no-fund

COPY browser-worker/server.mjs ./server.mjs
COPY browser-worker/entrypoint.sh ./entrypoint.sh

RUN mkdir -p /run/gpt-mirror && \
    chown -R pwuser:pwuser /app /run/gpt-mirror && \
    chmod 0755 /app/entrypoint.sh && \
    command -v node >/dev/null && \
    command -v Xvfb >/dev/null && \
    id pwuser >/dev/null

ENV BROWSER_SOCKET_PATH=/run/gpt-mirror/browser.sock
ENV BROWSER_NAVIGATION_TIMEOUT_MS=90000
ENV BROWSER_SEND_TIMEOUT_MS=1200000
ENV BROWSER_DISPLAY=99

ENTRYPOINT ["/app/entrypoint.sh"]
CMD ["node", "/app/server.mjs"]
