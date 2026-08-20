#!/usr/bin/env python3
"""
PipeWire / PulseAudio Smart Audio Router with Application Blacklisting.
Creates a virtual 'stream_sink' audio mix that includes:
- All desktop application audio (games, browsers, media players, system alerts)
- Optional microphone audio
EXCEPT applications explicitly listed in the blacklist (e.g., Discord, Slack, Spotify).
"""

import os
import re
import signal
import subprocess
import sys
import threading
import time

BLACKLIST_ENV = os.environ.get("AUDIO_BLACKLIST", "discord,Discord,vesktop,webcord,slack,zoom,teams")
INCLUDE_MIC = os.environ.get("INCLUDE_MIC", "false").lower() in ("true", "1", "yes")
STREAM_SINK_NAME = "stream_sink"

blacklist_terms = [t.strip().lower() for t in BLACKLIST_ENV.split(",") if t.strip()]
print(f"[AudioRouter] Active Audio Blacklist: {blacklist_terms}")
print(f"[AudioRouter] Include Microphone: {INCLUDE_MIC}")

module_id = None
running = True

def cleanup(signum=None, frame=None):
    global running, module_id
    running = False
    print("\n[AudioRouter] Shutting down audio router...")
    if module_id:
        try:
            subprocess.run(["pactl", "unload-module", str(module_id)], capture_output=True)
            print(f"[AudioRouter] Unloaded virtual sink module {module_id}")
        except Exception as e:
            print(f"[AudioRouter] Failed to unload module: {e}")
    sys.exit(0)

signal.signal(signal.SIGINT, cleanup)
signal.signal(signal.SIGTERM, cleanup)

def create_stream_sink():
    global module_id
    # Check if already exists
    res = subprocess.run(["pactl", "list", "short", "sinks"], capture_output=True, text=True)
    for line in res.stdout.splitlines():
        if STREAM_SINK_NAME in line:
            print(f"[AudioRouter] Virtual sink '{STREAM_SINK_NAME}' already exists.")
            return

    cmd = [
        "pactl", "load-module", "module-null-sink",
        f"sink_name={STREAM_SINK_NAME}",
        "sink_properties=device.description=Stream-Audio-Mix"
    ]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode == 0:
        module_id = res.stdout.strip()
        print(f"[AudioRouter] Created virtual sink '{STREAM_SINK_NAME}' (Module ID: {module_id})")
    else:
        print(f"[AudioRouter] Warning: Could not create sink via pactl: {res.stderr.strip()}")

def get_active_links():
    res = subprocess.run(["pw-link", "-l"], capture_output=True, text=True)
    links = set()
    for line in res.stdout.splitlines():
        if "->" in line:
            parts = [p.strip() for p in line.split("->")]
            if len(parts) == 2:
                links.add((parts[0], parts[1]))
    return links

def link_ports(src, dst):
    cmd = ["pw-link", src, dst]
    subprocess.run(cmd, capture_output=True)

def is_blacklisted(app_name, binary_name=""):
    name_check = (app_name + " " + binary_name).lower()
    return any(term in name_check for term in blacklist_terms)

def sync_audio_routes():
    # 1. Inspect all active audio outputs
    pactl_out = subprocess.run(["pactl", "list", "sink-inputs"], capture_output=True, text=True).stdout
    sink_inputs = pactl_out.split("Sink Input #")

    # Get available pw-link output ports
    pw_links = subprocess.run(["pw-link", "-o"], capture_output=True, text=True).stdout.splitlines()

    for block in sink_inputs:
        if not block.strip():
            continue
        
        # Parse app info
        app_name = ""
        app_binary = ""
        m_name = re.search(r'application\.name\s*=\s*"([^"]+)"', block)
        if m_name:
            app_name = m_name.group(1)
        m_bin = re.search(r'application\.process\.binary\s*=\s*"([^"]+)"', block)
        if m_bin:
            app_binary = m_bin.group(1)
        m_node = re.search(r'node\.name\s*=\s*"([^"]+)"', block)
        node_name = m_node.group(1) if m_node else app_name

        if not app_name and not node_name:
            continue

        if is_blacklisted(app_name, app_binary):
            # Blacklisted app: do NOT connect to stream sink
            # Ensure it is disconnected from stream_sink if accidentally connected
            for port in [f"{STREAM_SINK_NAME}:playback_FL", f"{STREAM_SINK_NAME}:playback_FR"]:
                for out_port in pw_links:
                    if app_name in out_port or app_binary in out_port or node_name in out_port:
                        subprocess.run(["pw-link", "-d", out_port, port], capture_output=True)
            continue

        # Allowed app: Link its output ports to stream_sink
        for out_port in pw_links:
            # Match application output ports
            if (app_name and app_name in out_port) or (app_binary and app_binary in out_port) or (node_name and node_name in out_port):
                if out_port.endswith("_FL") or out_port.endswith(":output_FL"):
                    link_ports(out_port, f"{STREAM_SINK_NAME}:playback_FL")
                elif out_port.endswith("_FR") or out_port.endswith(":output_FR"):
                    link_ports(out_port, f"{STREAM_SINK_NAME}:playback_FR")

    # 2. If INCLUDE_MIC is enabled, link default microphone source
    if INCLUDE_MIC:
        pw_inputs = subprocess.run(["pw-link", "-o"], capture_output=True, text=True).stdout.splitlines()
        for port in pw_inputs:
            if "capture_" in port and ("alsa_input" in port or "easyeffects_source" in port or "HyperX" in port or "denoised_source" in port):
                if port.endswith("_FL") or port.endswith("_MONO"):
                    link_ports(port, f"{STREAM_SINK_NAME}:playback_FL")
                if port.endswith("_FR") or port.endswith("_MONO"):
                    link_ports(port, f"{STREAM_SINK_NAME}:playback_FR")

def watch_events():
    """Watch pactl events in real-time to dynamically route newly launched applications."""
    proc = subprocess.Popen(
        ["pactl", "subscribe"],
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
        text=True
    )
    while running:
        line = proc.stdout.readline()
        if not line:
            break
        if "sink-input" in line or "client" in line or "source-output" in line:
            time.sleep(0.1) # Brief debounce for ports to register in PipeWire
            sync_audio_routes()

def main():
    create_stream_sink()
    print("[AudioRouter] Initializing audio routes...")
    sync_audio_routes()
    
    # Start event watcher thread
    t = threading.Thread(target=watch_events, daemon=True)
    t.start()
    
    print(f"[AudioRouter] Active and filtering audio! Monitoring for new streams...")
    while running:
        time.sleep(2)
        sync_audio_routes()

if __name__ == "__main__":
    main()
