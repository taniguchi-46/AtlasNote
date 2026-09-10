// Playwright CLI run-code --filename frontend/scripts/mermaid-browser-run.cjs
async page => {
  const origin = 'http://127.0.0.1:5187';
  const blocked = [];
  await page.route('**/*', route => {
    const request = route.request();
    const url = request.url();
    if (url === `${origin}/mermaid-regression`) {
      return route.fulfill({ contentType: 'text/html', body: '<!doctype html><title>Mermaid regression</title><body></body>' });
    }
    if (!url.startsWith(`${origin}/`) || request.resourceType() === 'image') {
      blocked.push(url);
      return route.abort();
    }
    return route.continue();
  });
  await page.goto(`${origin}/mermaid-regression`);
  const result = await page.evaluate(async () => {
    const checks = await import('/scripts/mermaid-browser-checks.mjs');
    return checks.runMermaidBrowserChecks();
  });
  if (blocked.length) throw new Error('Unexpected resource requests: ' + JSON.stringify(blocked));
  return { ...result, blockedRequests: blocked.length };
}
