package portal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

type StreamInfo struct {
	NodeID     int
	PipeWireFD int
	Width      int
	Height     int
}

type Client struct {
	conn          *dbus.Conn
	sessionHandle dbus.ObjectPath
	senderName    string
}

func NewClient() (*Client, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to session bus: %w", err)
	}

	sender := strings.ReplaceAll(strings.TrimPrefix(conn.Names()[0], ":"), ".", "_")

	return &Client{
		conn:       conn,
		senderName: sender,
	}, nil
}

func generateToken() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "u_" + hex.EncodeToString(b)
}

func (c *Client) RequestScreenCast() (*StreamInfo, error) {
	portal := c.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")

	// Step 1: CreateSession
	sessionToken := generateToken()
	createToken := generateToken()
	createReqPath := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", c.senderName, createToken))

	createSigChan := make(chan *dbus.Signal, 10)
	c.conn.Signal(createSigChan)

	rule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", createReqPath)
	c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	var sessionHandle dbus.ObjectPath
	var handleToken = createToken
	call := portal.Call("org.freedesktop.portal.ScreenCast.CreateSession", 0, map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(sessionToken),
		"handle_token":         dbus.MakeVariant(handleToken),
	})
	if call.Err != nil {
		return nil, fmt.Errorf("CreateSession DBus call failed: %w", call.Err)
	}

	for sig := range createSigChan {
		if sig.Path == createReqPath {
			if len(sig.Body) >= 2 {
				code := sig.Body[0].(uint32)
				if code != 0 {
					return nil, fmt.Errorf("CreateSession rejected with code %d", code)
				}
				results := sig.Body[1].(map[string]dbus.Variant)
				sessionHandle = dbus.ObjectPath(results["session_handle"].Value().(string))
				break
			}
		}
	}
	c.sessionHandle = sessionHandle

	// Step 2: SelectSources
	selectToken := generateToken()
	selectReqPath := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", c.senderName, selectToken))
	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", selectReqPath)
	c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	call = portal.Call("org.freedesktop.portal.ScreenCast.SelectSources", 0, sessionHandle, map[string]dbus.Variant{
		"types":        dbus.MakeVariant(uint32(3)), // 1=Monitor, 2=Window, 3=Both
		"multiple":     dbus.MakeVariant(false),
		"cursor_mode":  dbus.MakeVariant(uint32(2)), // Embedded cursor
		"handle_token": dbus.MakeVariant(selectToken),
	})
	if call.Err != nil {
		return nil, fmt.Errorf("SelectSources call failed: %w", call.Err)
	}

	for sig := range createSigChan {
		if sig.Path == selectReqPath {
			if len(sig.Body) >= 2 {
				code := sig.Body[0].(uint32)
				if code != 0 {
					return nil, fmt.Errorf("SelectSources rejected with code %d", code)
				}
				break
			}
		}
	}

	// Step 3: Start (Prompt on user desktop)
	startToken := generateToken()
	startReqPath := dbus.ObjectPath(fmt.Sprintf("/org/freedesktop/portal/desktop/request/%s/%s", c.senderName, startToken))
	rule = fmt.Sprintf("type='signal',interface='org.freedesktop.portal.Request',member='Response',path='%s'", startReqPath)
	c.conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, rule)

	fmt.Println("\n[*] >>> Please select your screen/window in the system dialog prompt <<<")
	call = portal.Call("org.freedesktop.portal.ScreenCast.Start", 0, sessionHandle, "", map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(startToken),
	})
	if call.Err != nil {
		return nil, fmt.Errorf("Start call failed: %w", call.Err)
	}

	var streamProps *StreamInfo
	for sig := range createSigChan {
		if sig.Path == startReqPath {
			if len(sig.Body) >= 2 {
				code := sig.Body[0].(uint32)
				if code != 0 {
					return nil, fmt.Errorf("user cancelled screen selection (code %d)", code)
				}
				results := sig.Body[1].(map[string]dbus.Variant)
				streamsVar := results["streams"].Value()
				var parseErr error
				streamProps, parseErr = ParseStreams(streamsVar)
				if parseErr != nil {
					return nil, parseErr
				}
				break
			}
		}
	}

	// Step 4: OpenPipeWireRemote
	var fd dbus.UnixFD
	call = portal.Call("org.freedesktop.portal.ScreenCast.OpenPipeWireRemote", 0, sessionHandle, map[string]dbus.Variant{})
	if call.Err != nil {
		return nil, fmt.Errorf("OpenPipeWireRemote failed: %w", call.Err)
	}
	if err := call.Store(&fd); err != nil {
		return nil, fmt.Errorf("failed to store PipeWire FD: %w", err)
	}

	streamProps.PipeWireFD = int(fd)
	return streamProps, nil
}

func ParseStreams(raw interface{}) (*StreamInfo, error) {
	switch v := raw.(type) {
	case [][]interface{}:
		if len(v) == 0 {
			return nil, errors.New("no stream returned in list")
		}
		nodeID := toInt(v[0][0])
		w, h := 3840, 2160
		if len(v[0]) > 1 {
			if props, ok := v[0][1].(map[string]interface{}); ok {
				if size, ok := props["size"].([]interface{}); ok && len(size) == 2 {
					w = toInt(size[0])
					h = toInt(size[1])
				}
			}
		}
		return &StreamInfo{NodeID: nodeID, Width: w, Height: h}, nil
	case []interface{}:
		if len(v) == 0 {
			return nil, errors.New("empty stream array")
		}
		if first, ok := v[0].([]interface{}); ok && len(first) > 0 {
			nodeID := toInt(first[0])
			w, h := 3840, 2160
			if len(first) > 1 {
				if props, ok := first[1].(map[string]interface{}); ok {
					if size, ok := props["size"].([]interface{}); ok && len(size) == 2 {
						w = toInt(size[0])
						h = toInt(size[1])
					}
				}
			}
			return &StreamInfo{NodeID: nodeID, Width: w, Height: h}, nil
		}
	}
	return &StreamInfo{NodeID: 0, Width: 3840, Height: 2160}, nil
}

func toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int32:
		return int(val)
	case int64:
		return int(val)
	case uint32:
		return int(val)
	case uint64:
		return int(val)
	}
	return 0
}

func (c *Client) Close() {
	if c.sessionHandle != "" {
		session := c.conn.Object("org.freedesktop.portal.Desktop", c.sessionHandle)
		_ = session.Call("org.freedesktop.portal.Session.Close", 0)
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
