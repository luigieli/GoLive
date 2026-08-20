#!/usr/bin/env python3
"""
Wayland XDG Desktop Portal ScreenCast & PipeWire Node Resolver.
Triggers the native system screen/window selection dialog (the same API used by Discord, OBS, Chrome).
Returns the PipeWire Node ID and File Descriptor for video capture.
"""

import os
import sys
import uuid
import dbus
from dbus.mainloop.glib import DBusGMainLoop
from gi.repository import GLib

def get_screencast_stream():
    DBusGMainLoop(set_as_default=True)
    bus = dbus.SessionBus()
    loop = GLib.MainLoop()

    portal = bus.get_object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
    screencast = dbus.Interface(portal, "org.freedesktop.portal.ScreenCast")

    sender_name = bus.get_unique_name()[1:].replace(".", "_")
    
    state = {
        "session_handle": None,
        "node_id": None,
        "pipewire_fd": None,
        "error": None
    }

    def on_session_response(response, results):
        if response != 0:
            state["error"] = f"Failed to create portal session (code {response})"
            loop.quit()
            return
        state["session_handle"] = str(results.get("session_handle", ""))
        
        # Step 2: Select Sources
        select_token = f"u_{uuid.uuid4().hex[:8]}"
        select_req_path = f"/org/freedesktop/portal/desktop/request/{sender_name}/{select_token}"
        bus.add_signal_receiver(
            on_select_response,
            signal_name="Response",
            path=select_req_path,
            dbus_interface="org.freedesktop.portal.Request"
        )
        
        # types: 1=Monitor, 2=Window, 3=Both
        # cursor_mode: 1=Hidden, 2=Embedded, 4=Metadata
        screencast.SelectSources(
            state["session_handle"],
            {
                "types": dbus.UInt32(3),
                "multiple": dbus.Boolean(False),
                "cursor_mode": dbus.UInt32(2),
                "handle_token": select_token
            }
        )

    def on_select_response(response, results):
        if response != 0:
            state["error"] = f"User cancelled screen selection or portal error (code {response})"
            loop.quit()
            return

        # Step 3: Start session (triggers Hyprland/Wayland picker UI)
        start_token = f"u_{uuid.uuid4().hex[:8]}"
        start_req_path = f"/org/freedesktop/portal/desktop/request/{sender_name}/{start_token}"
        bus.add_signal_receiver(
            on_start_response,
            signal_name="Response",
            path=start_req_path,
            dbus_interface="org.freedesktop.portal.Request"
        )
        screencast.Start(
            state["session_handle"],
            "",
            {"handle_token": start_token}
        )

    def on_start_response(response, results):
        if response != 0:
            state["error"] = f"Screen selection cancelled or failed (code {response})"
            loop.quit()
            return
        
        streams = results.get("streams", [])
        if not streams:
            state["error"] = "No stream selected by user"
            loop.quit()
            return

        # streams is array of (node_id, options)
        node_id = int(streams[0][0])
        state["node_id"] = node_id
        
        # Step 4: Open PipeWire Remote File Descriptor
        try:
            fd_handle = screencast.OpenPipeWireRemote(state["session_handle"], {})
            state["pipewire_fd"] = fd_handle.take()
        except Exception as e:
            state["error"] = f"Failed to open PipeWire remote: {e}"
        
        loop.quit()

    # Step 1: Create Session
    session_token = f"u_{uuid.uuid4().hex[:8]}"
    create_token = f"u_{uuid.uuid4().hex[:8]}"
    create_req_path = f"/org/freedesktop/portal/desktop/request/{sender_name}/{create_token}"
    
    bus.add_signal_receiver(
        on_session_response,
        signal_name="Response",
        path=create_req_path,
        dbus_interface="org.freedesktop.portal.Request"
    )

    screencast.CreateSession({
        "session_handle_token": session_token,
        "handle_token": create_token
    })

    # Run DBus event loop
    loop.run()

    if state["error"]:
        print(f"Error: {state['error']}", file=sys.stderr)
        sys.exit(1)

    return state["node_id"], state["pipewire_fd"], state["session_handle"]

if __name__ == "__main__":
    node_id, fd, session = get_screencast_stream()
    print(f"NODE_ID={node_id}")
    print(f"PIPEWIRE_FD={fd}")
    print(f"SESSION_HANDLE={session}")
