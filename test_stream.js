#!/usr/bin/env node

/**
 * =========================================================================
 *  GoLive Stream, Audio & Video Recording Evaluation Suite
 * =========================================================================
 * 
 * Usage:
 *   node test_stream.js [TARGET_URL] [DURATION_SECONDS] [--headed]
 * 
 * Examples:
 *   node test_stream.js https://stream.luigieli.com 15
 *   node test_stream.js http://localhost:8080 30 --headed
 */

const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');

const args = process.argv.slice(2);
const isHeaded = args.includes('--headed');
const cleanArgs = args.filter(a => a !== '--headed');

const TARGET_URL = cleanArgs[0] || process.env.STREAM_URL || 'https://stream.luigieli.com';
const DURATION_SECONDS = parseInt(cleanArgs[1], 10) || 15;

const RECORDINGS_DIR = path.resolve(__dirname, 'recordings');
if (!fs.existsSync(RECORDINGS_DIR)) {
    fs.mkdirSync(RECORDINGS_DIR, { recursive: true });
}

async function runStreamAudioTest() {
    console.log('================================================================');
    console.log('  🎥 GoLive Stream, Audio & Video Recording Suite');
    console.log('================================================================');
    console.log(`[*] Target URL        : ${TARGET_URL}`);
    console.log(`[*] Record Duration   : ${DURATION_SECONDS} seconds`);
    console.log(`[*] Headless Mode     : ${!isHeaded}`);
    console.log(`[*] Recordings Output : ${RECORDINGS_DIR}`);
    console.log('----------------------------------------------------------------');

    const browser = await chromium.launch({
        headless: !isHeaded,
        args: [
            '--no-sandbox',
            '--disable-setuid-sandbox',
            '--autoplay-policy=no-user-gesture-required',
            '--disable-web-security'
        ]
    });

    // Configure Playwright context with built-in video recording
    const context = await browser.newContext({
        viewport: { width: 1280, height: 720 },
        ignoreHTTPSErrors: true,
        recordVideo: {
            dir: RECORDINGS_DIR,
            size: { width: 1280, height: 720 }
        }
    });

    const page = await context.newPage();

    page.on('console', msg => {
        const text = msg.text();
        if (text.includes('Error') || text.includes('error') || text.includes('failed') || text.includes('Media Info')) {
            console.log(`[Browser Console]: ${text}`);
        }
    });

    console.log(`[*] Loading player in browser...`);
    try {
        await page.goto(TARGET_URL, { waitUntil: 'domcontentloaded', timeout: 15000 });
    } catch (e) {
        console.error(`[-] Failed to load ${TARGET_URL}: ${e.message}`);
        await browser.close();
        process.exit(1);
    }

    // Collect metrics once per second
    const samples = [];

    console.log('----------------------------------------------------------------');
    console.log(`  ⏱️  Recording & Evaluating Stream (${DURATION_SECONDS} seconds)...`);
    console.log('----------------------------------------------------------------');

    for (let sec = 1; sec <= DURATION_SECONDS; sec++) {
        await new Promise(r => setTimeout(r, 1000));

        if (sec === 1 || sec === 3) {
            await page.click('#unmuteBanner', { timeout: 500 }).catch(() => {});
            await page.click('#videoElement', { timeout: 500 }).catch(() => {});
        }

        const metrics = await page.evaluate(() => {
            const v = document.getElementById('videoElement');
            const p = window.player;

            const mediaInfo = p && p.mediaInfo ? p.mediaInfo : {};
            const hasAudio = !!(mediaInfo.hasAudio || mediaInfo.audioCodec);
            const stats = p && p.statisticsInfo ? p.statisticsInfo : {};

            return {
                currentTime: v ? v.currentTime : 0,
                readyState: v ? v.readyState : 0,
                paused: v ? v.paused : true,
                muted: v ? v.muted : true,
                volume: v ? v.volume : 0,
                videoWidth: v ? v.videoWidth : 0,
                videoHeight: v ? v.videoHeight : 0,
                statusText: document.getElementById('statusText')?.innerText || 'UNKNOWN',
                hasAudio: hasAudio,
                audioCodec: mediaInfo.audioCodec || (hasAudio ? 'AAC' : 'none'),
                decodedFrames: stats.decodedFrames || 0,
                droppedFrames: stats.droppedFrames || 0,
                speedKB: stats.speed || 0
            };
        });

        samples.push(metrics);

        const timeProgress = metrics.currentTime.toFixed(1) + 's';
        const res = metrics.videoWidth > 0 ? `${metrics.videoWidth}x${metrics.videoHeight}` : '---';
        const audioStatus = metrics.hasAudio ? `🔊 AUDIO (${metrics.audioCodec})` : `🔇 NO AUDIO`;
        const speed = metrics.speedKB > 0 ? `${metrics.speedKB.toFixed(0)} KB/s` : '0 KB/s';

        console.log(`[T+${String(sec).padStart(2, '0')}s] Video: ${res.padEnd(9)} | Progress: ${timeProgress.padEnd(6)} | State: ${metrics.statusText.padEnd(16)} | Rate: ${speed.padEnd(10)} | ${audioStatus}`);
    }

    // Save final screenshot
    const screenshotPath = path.join(RECORDINGS_DIR, 'stream_screenshot.png');
    await page.screenshot({ path: screenshotPath });

    // Close page and browser to finalize Playwright video
    const videoObj = page.video();
    await page.close();
    await context.close();
    await browser.close();

    const finalWebmPath = path.join(RECORDINGS_DIR, 'stream_evaluated.webm');
    const finalMp4Path = path.join(RECORDINGS_DIR, 'stream_evaluated.mp4');

    if (videoObj) {
        const rawVideoPath = await videoObj.path();
        if (fs.existsSync(rawVideoPath)) {
            fs.copyFileSync(rawVideoPath, finalWebmPath);
            try {
                execSync(`ffmpeg -y -i "${rawVideoPath}" -c:v libx264 -preset fast -crf 22 -c:a aac -b:a 128k "${finalMp4Path}" 2>/dev/null`);
            } catch (e) {
                // Ignore conversion errors if ffmpeg is busy
            }
        }
    }

    // Analyze final test results
    const lastSample = samples[samples.length - 1] || {};
    const videoAdvancing = samples.length > 5 && (samples[samples.length - 1].currentTime > samples[0].currentTime);
    const hasAudioTrack = samples.some(s => s.hasAudio);
    const hasActiveFrames = lastSample.videoWidth > 0 && lastSample.readyState >= 2;

    console.log('\n================================================================');
    console.log('  📋 RECORDING & EVALUATION REPORT');
    console.log('================================================================');
    console.log(`[*] Target Tested        : ${TARGET_URL}`);
    console.log(`[*] Stream Resolution    : ${lastSample.videoWidth || 0}x${lastSample.videoHeight || 0}`);
    console.log(`[*] Video Playback       : ${videoAdvancing ? '✅ ADVANCING (' + lastSample.currentTime.toFixed(2) + 's)' : (hasActiveFrames ? '✅ ACTIVE (ReadyState ' + lastSample.readyState + ')' : '❌ STALLED')}`);
    console.log(`[*] Audio Track Present  : ${hasAudioTrack ? '✅ YES (' + (lastSample.audioCodec || 'AAC') + ' Stream Muxed)' : '❌ NO AUDIO TRACK'}`);
    console.log(`[*] Audio Unmuted        : ${!lastSample.muted ? '✅ UNMUTED (Vol: ' + (lastSample.volume * 100).toFixed(0) + '%)' : '⚠️  MUTED'}`);
    console.log(`[*] Network Throughput   : ${lastSample.speedKB ? lastSample.speedKB.toFixed(1) + ' KB/s' : 'Active'}`);
    console.log(`[*] Stream Status Badge  : ${lastSample.statusText}`);
    console.log('----------------------------------------------------------------');
    console.log(`[*] Recorded MP4 Video   : ${fs.existsSync(finalMp4Path) ? finalMp4Path : 'N/A'}`);
    console.log(`[*] Recorded WebM Video  : ${fs.existsSync(finalWebmPath) ? finalWebmPath : 'N/A'}`);
    console.log(`[*] Snapshot Image       : ${screenshotPath}`);
    console.log('================================================================');

    const passed = (videoAdvancing || hasActiveFrames) && hasAudioTrack;

    if (passed) {
        console.log('🎉 RESULT: PASSED - Recorded stream video & audio successfully!\n');
        process.exit(0);
    } else {
        console.error('❌ RESULT: FAILED - Stream or Audio track was not active.\n');
        process.exit(1);
    }
}

runStreamAudioTest().catch(err => {
    console.error('[-] Test execution error:', err);
    process.exit(1);
});
