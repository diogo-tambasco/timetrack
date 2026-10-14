// Gera os ícones PWA placeholder (glifo de relógio teal do brand) usando o
// chromium do Playwright. Rodar de frontend/: node ../scripts/gen-icons.mjs
import { chromium } from '@playwright/test';
import { mkdirSync } from 'node:fs';

const svg = (size) => `<!doctype html><html><body style="margin:0">
<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 24 24">
  <rect width="24" height="24" rx="6" fill="#2dd4bf"/>
  <circle cx="12" cy="12" r="6.5" fill="none" stroke="#04221d" stroke-width="1.8"/>
  <path d="M12 8.5V12l2.4 1.6" fill="none" stroke="#04221d" stroke-width="1.8" stroke-linecap="round"/>
</svg></body></html>`;

mkdirSync('public/icons', { recursive: true });
const browser = await chromium.launch();
for (const size of [192, 512]) {
  const page = await browser.newPage({ viewport: { width: size, height: size } });
  await page.setContent(svg(size));
  await page.screenshot({ path: `public/icons/icon-${size}.png`, clip: { x: 0, y: 0, width: size, height: size } });
  await page.close();
}
await browser.close();
console.log('ícones gerados em public/icons/');
