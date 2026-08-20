import { chromium } from 'playwright';

async function run() {
    console.log("Launching Chrome browser at /usr/bin/google-chrome-stable...");
    const browser = await chromium.launch({
        executablePath: '/usr/bin/google-chrome-stable',
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu']
    });
    const page = await browser.newPage();

    console.log("=== Network Activity Log on http://localhost:8080 ===");

    page.on('request', req => {
        console.log(`--> [REQ] ${req.method()} ${req.url()}`);
    });

    page.on('response', resp => {
        console.log(`<-- [RESP ${resp.status()}] ${resp.url()} (Content-Type: ${resp.headers()['content-type'] || 'none'})`);
    });

    page.on('console', msg => {
        console.log(`[CONSOLE ${msg.type().toUpperCase()}] ${msg.text()}`);
    });

    page.on('pageerror', err => {
        console.log(`[PAGE ERROR] ${err.message}`);
    });

    try {
        console.log("Navigating to http://localhost:8080/ ...");
        const res = await page.goto('http://localhost:8080/', { waitUntil: 'networkidle', timeout: 8000 });
        console.log(`Main Page Status: ${res ? res.status() : 'N/A'}`);

        console.log("Waiting 6 seconds to observe HLS polling and streaming...");
        await new Promise(r => setTimeout(r, 6000));
    } catch (e) {
        console.log("Note:", e.message);
    } finally {
        await browser.close();
        console.log("=== End of Network Trace ===");
    }
}

run().catch(console.error);
