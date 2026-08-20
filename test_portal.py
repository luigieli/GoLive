#!/usr/bin/env python3
import os
import subprocess
import sys
import threading
import time
import uuid
import dbus
from dbus.mainloop.glib import DBusGMainLoop
from gi.repository import GLib

DBusGMainLoop(set_as_default=True)
bus = dbus.SessionBus()
loop = GLib.MainLoop()

portal = bus.get_object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
screencast = dbus.Interface(portal, "org.freedesktop.portal.ScreenCast")
sender_name = bus.get_unique_name()[1:].replace(".", "_")

session_handle = None
node_id = None
pipewire_fd = None
ready = threading.Event()

def on_session_response(resp, results):
    global session_handle
    if resp != 0:
        print(f"Session failed: {resp}")
        ready.set()
        return
    session_handle = str(results.get("session_handle", ""))
    print(f"Session created: {session_handle}")
    
    token = f"u_{uuid.uuid4().hex[:8]}"
    bus.add_signal_receiver(
        on_select_response,
        signal_name="Response",
        path=f"/org/freedesktop/portal/desktop/request/{sender_name}/{token}",
        dbus_interface="org.freedesktop.portal.Request"
    )
    screencast.SelectSources(
        session_handle,
        {"types": dbus.UInt32(3), "multiple": dbus.Boolean(False), "cursor_mode": dbus.UInt32(2), "handle_token": token}
    )

def on_select_response(resp, results):
    if resp != 0:
        print(f"Select failed: {resp}")
        ready.set()
        return
    
    token = f"u_{uuid.uuid4().hex[:8]}"
    bus.add_signal_receiver(
        on_start_response,
        signal_name="Response",
        path=f"/org/freedesktop/portal/desktop/request/{sender_name}/{token}",
        dbus_interface="org.freedesktop.portal.Request"
    )
    print(">>> PLEASE SELECT SCREEN IN PORTAL DIALOG <<<")
    screencast.Start(session_handle, "", {"handle_token": token})

def on_start_response(resp, results):
    global node_id, pipewire_fd
    if resp != 0:
        print(f"Start failed: {resp}")
        ready.set()
        return
    streams = results.get("streams", [])
    print(f"Streams: {streams}")
    node_id = int(streams[0][0])
    
    fd_handle = screencast.OpenPipeWireRemote(session_handle, {})
    pipewire_fd = fd_handle.take()
    print(f"Node ID: {node_id}, FD: {pipewire_fd}")
    ready.set()

threading.Thread(target=loop.run, daemon=True).start()

session_token = f"u_{uuid.uuid4().hex[:8]}"
create_token = f"u_{uuid.uuid4().hex[:8]}"
bus.add_signal_receiver(
    on_session_response,
    signal_name="Response",
    path=f"/org/freedesktop/portal/desktop/request/{sender_name}/{create_token}",
    dbus_interface="org.freedesktop.portal.Request"
)
screencast.CreateSession({"session_handle_token": session_token, "handle_token": create_token})

ready.wait()

if pipewire_fd:
    print("\nTesting GStreamer pipeline with GST_DEBUG=pipewiresrc:5,videoconvert:3...")
    gst_cmd = [
        "gst-launch-1.0", "-v",
        "pipewiresrc", f"fd={pipewire_fd}", f"path={node_id}",
        "do-timestamp=true", "keepalive-time=33",
        "!", "videoconvert",
        "!", "fakesink", "num-buffers=10"
    ]
    env = os.environ.copy()
    env["GST_DEBUG"] = "pipewiresrc:4"
    proc = subprocess.run(gst_cmd, pass_fds=[pipewire_fd], env=env)
    print(f"GStreamer exit code: {proc.returncode}")
