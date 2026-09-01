FROM mcr.microsoft.com/playwright:v1.62.1-noble

WORKDIR /app

COPY browser-worker/package.json ./package.json
RUN PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 npm install --omit=dev --no-audit --no-fund

COPY browser-worker/server.mjs ./server.mjs

RUN mkdir -p /run/gpt-mirror && chown -R pwuser:pwuser /app /run/gpt-mirror

USER pwuser
ENV BROWSER_SOCKET_PATH=/run/gpt-mirror/browser.sock
ENV BROWSER_NAVIGATION_TIMEOUT_MS=90000
ENV BROWSER_SEND_TIMEOUT_MS=1200000

CMD ["xvfb-run", "-a", "node", "/app/server.mjs"]
