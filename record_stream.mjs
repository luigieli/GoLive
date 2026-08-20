import { chromium } from 'playwright';
import path from 'path';
import fs from 'fs';

const videoDir = '/home/luigi/.gemini/antigravity-cli/brain/b6fbcb1c-4f49-42b8-b191-ebb5db85f7ed/videos';

async function main() {
    console.log("[Playwright] Launching Chrome with video recording enabled...");
    const browser = await chromium.launch({
        executablePath: '/usr/bin/google-chrome-stable',
        headless: true,
        args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu', '--autoplay-policy=no-user-gesture-required']
    });

    const context = await browser.newContext({
        recordVideo: {
            dir: videoDir,
            size: { width: 1280, height: 720 }
        },
        viewport: { width: 1280, height: 720 }
    });

    const page = await context.newPage();

    page.on('console', msg => console.log(`[Browser Console] ${msg.type()}: ${msg.text()}`));
    page.on('request', req => {
        if (req.url().includes('/hls/')) {
            console.log(`[HLS Request] ${req.url()}`);
        }
    });
    page.on('response', resp => {
        if (resp.url().includes('/hls/')) {
            console.log(`[HLS Response ${resp.status()}] ${resp.url()}`);
        }
    });

    console.log("[Playwright] Navigating to http://localhost:8080/ ...");
    await page.goto('http://localhost:8080/', { waitUntil: 'networkidle', timeout: 15000 });

    console.log("[Playwright] Waiting for video element to start playing...");
    await page.waitForFunction(() => {
        const video = document.querySelector('video');
        return video && !video.paused && video.currentTime > 0;
    }, { timeout: 30000 }).catch(() => console.log("[Playwright] Warning: Video autoplay timeout, continuing recording..."));

    console.log("[Playwright] Recording 30 seconds of continuous live stream delivery...");
    // Record for 30 seconds
    await new Promise(r => setTimeout(r, 30000));

    console.log("[Playwright] Closing page to finalize video recording...");
    await page.close();
    await context.close();
    await browser.close();

    // Find the saved video file
    const files = fs.readdirSync(videoDir).filter(f => f.endsWith('.webm'));
    if (files.length > 0) {
        // Sort by mtime
        files.sort((a, b) => fs.statSync(path.join(videoDir, b)).mtimeMs - fs.statSync(path.join(videoDir, a)).mtimeMs);
        const latestVideo = path.join(videoDir, files[0]);
        console.log(`[Playwright] Video recording saved successfully: ${latestVideo}`);
        const destVideo = '/home/luigi/.gemini/antigravity-cli/brain/b6fbcb1c-4f49-42b8-b191-ebb5db85f7ed/stream_recording.webm';
        fs.copyFileSync(latestVideo, destVideo);
        console.log(`[Playwright] Copied to artifact destination: ${destVideo}`);
    }
}

main().catch(err => {
    console.error("[Playwright] Error during recording:", err);
    process.exit(1);
});
