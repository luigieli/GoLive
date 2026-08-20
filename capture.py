#!/usr/bin/env python3
"""
Wayland PipeWire Native Screen Capture & FFmpeg HLS Transcoder.
Uses PyGObject (Gst) with appsink to pull YUV420p buffers in real-time
and stream directly into FFmpeg HLS muxer.
"""

import os
import signal
import subprocess
import sys
import threading
import time
import uuid
import dbus
from dbus.mainloop.glib import DBusGMainLoop
import gi
gi.require_version('Gst', '1.0')
from gi.repository import GLib, Gst

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
HLS_DIR = os.path.join(SCRIPT_DIR, "hls")
os.makedirs(HLS_DIR, exist_ok=True)

FRAMERATE = int(os.environ.get("FRAMERATE", "30"))
VIDEO_BITRATE = os.environ.get("VIDEO_BITRATE", "6000")
VIDEO_BITRATE_NUM = "".join(filter(str.isdigit, VIDEO_BITRATE)) or "6000"
HLS_TIME = os.environ.get("HLS_TIME", "2")
HLS_LIST_SIZE = os.environ.get("HLS_LIST_SIZE", "5")
AUDIO_SOURCE = os.environ.get("AUDIO_SOURCE", "stream_sink.monitor")

# Initialize GStreamer & DBus GLib Loop
DBusGMainLoop(set_as_default=True)
Gst.init(None)

class ScreenCastSession:
    def __init__(self, loop):
        self.bus = dbus.SessionBus()
        self.loop = loop
        
        self.portal = self.bus.get_object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
        self.screencast = dbus.Interface(self.portal, "org.freedesktop.portal.ScreenCast")
        self.sender_name = self.bus.get_unique_name()[1:].replace(".", "_")
        
        self.session_handle = None
        self.node_id = None
        self.pipewire_fd = None
        self.width = 3840
        self.height = 2160
        self.error = None
        self.ready_event = threading.Event()

    def start(self):
        session_token = f"u_{uuid.uuid4().hex[:8]}"
        create_token = f"u_{uuid.uuid4().hex[:8]}"
        create_req_path = f"/org/freedesktop/portal/desktop/request/{self.sender_name}/{create_token}"
        
        self.bus.add_signal_receiver(
            self.on_session_response,
            signal_name="Response",
            path=create_req_path,
            dbus_interface="org.freedesktop.portal.Request"
        )

        self.screencast.CreateSession({
            "session_handle_token": session_token,
            "handle_token": create_token
        })

        self.ready_event.wait()

        if self.error:
            print(f"[!] Portal Error: {self.error}", file=sys.stderr, flush=True)
            sys.exit(1)

        return self.node_id, self.pipewire_fd, self.width, self.height

    def on_session_response(self, response, results):
        if response != 0:
            self.error = f"Failed to create portal session (code {response})"
            self.ready_event.set()
            return
        self.session_handle = str(results.get("session_handle", ""))
        
        select_token = f"u_{uuid.uuid4().hex[:8]}"
        select_req_path = f"/org/freedesktop/portal/desktop/request/{self.sender_name}/{select_token}"
        self.bus.add_signal_receiver(
            self.on_select_response,
            signal_name="Response",
            path=select_req_path,
            dbus_interface="org.freedesktop.portal.Request"
        )
        self.screencast.SelectSources(
            self.session_handle,
            {
                "types": dbus.UInt32(3),       # 1=Monitor, 2=Window, 3=Both
                "multiple": dbus.Boolean(False),
                "cursor_mode": dbus.UInt32(2),  # 2=Embedded cursor
                "handle_token": select_token
            }
        )

    def on_select_response(self, response, results):
        if response != 0:
            self.error = f"User cancelled screen selection or portal error (code {response})"
            self.ready_event.set()
            return

        start_token = f"u_{uuid.uuid4().hex[:8]}"
        start_req_path = f"/org/freedesktop/portal/desktop/request/{self.sender_name}/{start_token}"
        self.bus.add_signal_receiver(
            self.on_start_response,
            signal_name="Response",
            path=start_req_path,
            dbus_interface="org.freedesktop.portal.Request"
        )
        print("\n[*] >>> Please select your screen/window in the system dialog prompt <<<", flush=True)
        self.screencast.Start(
            self.session_handle,
            "",
            {"handle_token": start_token}
        )

    def on_start_response(self, response, results):
        if response != 0:
            self.error = f"Screen selection cancelled or failed (code {response})"
            self.ready_event.set()
            return
        
        streams = results.get("streams", [])
        if not streams:
            self.error = "No stream selected by user"
            self.ready_event.set()
            return

        self.node_id = int(streams[0][0])
        props = streams[0][1] if len(streams[0]) > 1 else {}
        if "size" in props:
            sz = props["size"]
            self.width = int(sz[0])
            self.height = int(sz[1])
        
        try:
            fd_handle = self.screencast.OpenPipeWireRemote(self.session_handle, {})
            self.pipewire_fd = fd_handle.take()
        except Exception as e:
            self.error = f"Failed to open PipeWire remote: {e}"
        
        self.ready_event.set()

    def close(self):
        if self.session_handle:
            try:
                session_obj = self.bus.get_object("org.freedesktop.portal.Desktop", self.session_handle)
                session_iface = dbus.Interface(session_obj, "org.freedesktop.portal.Session")
                session_iface.Close()
            except Exception:
                pass

def run_pipeline(session, node_id, pipewire_fd, width, height):
    print(f"[*] Stream Selected! (PipeWire Node ID: {node_id}, FD: {pipewire_fd}, Resolution: {width}x{height})", flush=True)
    print(f"[*] Audio Source: {AUDIO_SOURCE}", flush=True)
    print(f"[*] Starting HLS Encoder pipeline...", flush=True)

    # Clean previous segments
    for f in os.listdir(HLS_DIR):
        if f.endswith(".ts") or f.endswith(".m3u8"):
            try:
                os.remove(os.path.join(HLS_DIR, f))
            except Exception:
                pass

    gop_size = FRAMERATE * 2

    audio_flags = []
    if AUDIO_SOURCE != "none":
        audio_flags = ["-f", "pulse", "-i", AUDIO_SOURCE, "-c:a", "aac", "-b:a", "192k", "-ar", "48000"]
    else:
        audio_flags = ["-an"]

    manifest_path = os.path.join(HLS_DIR, "index.m3u8")
    segment_pattern = os.path.join(HLS_DIR, "stream_%04d.ts")

    ffmpeg_cmd = [
        "ffmpeg", "-hide_banner", "-loglevel", "info",
        "-f", "rawvideo",
        "-pix_fmt", "yuv420p",
        "-s", f"{width}x{height}",
        "-r", str(FRAMERATE),
        "-i", "-",
        *audio_flags,
        "-c:v", "libx264",
        "-preset", "veryfast",
        "-tune", "zerolatency",
        "-profile:v", "high",
        "-pix_fmt", "yuv420p",
        "-b:v", f"{VIDEO_BITRATE_NUM}k",
        "-g", str(gop_size),
        "-keyint_min", str(FRAMERATE),
        "-sc_threshold", "0",
        "-f", "hls",
        "-hls_init_time", "1",
        "-hls_time", str(HLS_TIME),
        "-hls_list_size", str(HLS_LIST_SIZE),
        "-hls_flags", "delete_segments+omit_endlist",
        "-hls_segment_type", "mpegts",
        "-hls_segment_filename", segment_pattern,
        manifest_path
    ]

    ffmpeg_proc = subprocess.Popen(
        ffmpeg_cmd,
        stdin=subprocess.PIPE,
        stdout=sys.stdout,
        stderr=sys.stderr
    )

    # In-process GStreamer pipeline with appsink and videoscale
    gst_pipeline_str = (
        f"pipewiresrc name=src do-timestamp=true keepalive-time=33 always-copy=true ! "
        f"videoconvert ! videoscale ! videorate ! "
        f"video/x-raw,format=I420,width={width},height={height},framerate={FRAMERATE}/1 ! "
        f"appsink name=sink emit-signals=true max-buffers=4 drop=true sync=false"
    )

    pipeline = Gst.parse_launch(gst_pipeline_str)
    src = pipeline.get_by_name("src")
    src.set_property("fd", pipewire_fd)
    src.set_property("path", str(node_id))

    sink = pipeline.get_by_name("sink")
    
    frame_count = [0]
    first_frame = [False]

    def on_new_sample(app_sink):
        sample = app_sink.emit("pull-sample")
        if not sample:
            return Gst.FlowReturn.OK
        buf = sample.get_buffer()
        success, map_info = buf.map(Gst.MapFlags.READ)
        if success:
            try:
                ffmpeg_proc.stdin.write(map_info.data)
                ffmpeg_proc.stdin.flush()
                frame_count[0] += 1
                if not first_frame[0]:
                    first_frame[0] = True
                    print("[*] First video frame received & sent to FFmpeg encoder!", flush=True)
            except (BrokenPipeError, OSError):
                pass
            finally:
                buf.unmap(map_info)
        return Gst.FlowReturn.OK

    sink.connect("new-sample", on_new_sample)

    bus = pipeline.get_bus()
    bus.add_signal_watch()
    def on_bus_message(bus, msg):
        if msg.type == Gst.MessageType.ERROR:
            err, debug = msg.parse_error()
            print(f"[!] GStreamer Error: {err}: {debug}", file=sys.stderr, flush=True)
        elif msg.type == Gst.MessageType.STATE_CHANGED and msg.src == pipeline:
            old, new, pending = msg.parse_state_changed()
            if new == Gst.State.PLAYING:
                print("[*] GStreamer pipeline is PLAYING! Waiting for screen frames...", flush=True)

    bus.connect("message", on_bus_message)
    pipeline.set_state(Gst.State.PLAYING)

    def handle_sig(sig, frame):
        print("\n[*] Stopping pipeline...", flush=True)
        try:
            pipeline.set_state(Gst.State.NULL)
        except Exception:
            pass
        ffmpeg_proc.terminate()
        try:
            os.close(pipewire_fd)
        except Exception:
            pass
        session.close()
        sys.exit(0)

    signal.signal(signal.SIGINT, handle_sig)
    signal.signal(signal.SIGTERM, handle_sig)

    ffmpeg_proc.wait()
    pipeline.set_state(Gst.State.NULL)
    session.close()

if __name__ == "__main__":
    loop = GLib.MainLoop()
    threading.Thread(target=loop.run, daemon=True).start()
    session = ScreenCastSession(loop)
    node_id, fd, width, height = session.start()
    run_pipeline(session, node_id, fd, width, height)
