package portal

import (
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/luigieli/streaming/utils/crypto"
	"github.com/luigieli/streaming/utils/types"
)

type StreamInfo = types.StreamInfo

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

func (c *Client) RequestScreenCast() (*StreamInfo, error) {
	portal := c.conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")

	// Step 1: CreateSession
	sessionToken := crypto.GenerateToken()
	createToken := crypto.GenerateToken()
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
	selectToken := crypto.GenerateToken()
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
	startToken := crypto.GenerateToken()
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

	if streamProps == nil {
		return nil, errors.New("failed to acquire stream info from portal")
	}

	// Step 4: OpenPipeWireRemote
	call = portal.Call("org.freedesktop.portal.ScreenCast.OpenPipeWireRemote", 0, sessionHandle, map[string]dbus.Variant{})
	if call.Err != nil {
		return nil, fmt.Errorf("OpenPipeWireRemote failed: %w", call.Err)
	}

	if len(call.Body) == 0 {
		return nil, errors.New("OpenPipeWireRemote returned empty response")
	}

	fd, ok := call.Body[0].(dbus.UnixFD)
	if !ok {
		return nil, fmt.Errorf("expected UnixFD, got %T", call.Body[0])
	}
	streamProps.PipeWireFD = int(fd)

	return streamProps, nil
}

func (c *Client) Close() {
	if c.conn != nil {
		if c.sessionHandle != "" {
			session := c.conn.Object("org.freedesktop.portal.Desktop", c.sessionHandle)
			_ = session.Call("org.freedesktop.portal.Session.Close", 0)
		}
		_ = c.conn.Close()
	}
}

func ParseStreams(streamsVar interface{}) (*StreamInfo, error) {
	switch val := streamsVar.(type) {
	case [][]interface{}:
		if len(val) == 0 || len(val[0]) < 2 {
			return nil, errors.New("invalid streams array format")
		}
		nodeID := int(val[0][0].(uint32))
		props := val[0][1].(map[string]interface{})
		width, height := parseStreamProperties(props)
		return &StreamInfo{
			NodeID: nodeID,
			Width:  width,
			Height: height,
		}, nil
	case []interface{}:
		if len(val) == 0 {
			return nil, errors.New("empty streams list")
		}
		tuple, ok := val[0].([]interface{})
		if !ok || len(tuple) < 2 {
			if structVal, ok := val[0].(struct {
				NodeID uint32
				Props  map[string]dbus.Variant
			}); ok {
				width, height := parseVariantMap(structVal.Props)
				return &StreamInfo{
					NodeID: int(structVal.NodeID),
					Width:  width,
					Height: height,
				}, nil
			}
			return nil, errors.New("unrecognized stream tuple format")
		}
		nodeID := int(tuple[0].(uint32))
		props := tuple[1].(map[string]interface{})
		width, height := parseStreamProperties(props)
		return &StreamInfo{
			NodeID: nodeID,
			Width:  width,
			Height: height,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected streams type %T", streamsVar)
	}
}

func parseVariantMap(props map[string]dbus.Variant) (int, int) {
	if sizeVar, ok := props["size"]; ok {
		if sizeSlice, ok := sizeVar.Value().([]interface{}); ok && len(sizeSlice) >= 2 {
			return int(sizeSlice[0].(int32)), int(sizeSlice[1].(int32))
		}
	}
	return 1920, 1080
}

func parseStreamProperties(props map[string]interface{}) (int, int) {
	if sizeVar, ok := props["size"]; ok {
		switch s := sizeVar.(type) {
		case []interface{}:
			if len(s) >= 2 {
				return toInt(s[0]), toInt(s[1])
			}
		case []uint32:
			if len(s) >= 2 {
				return int(s[0]), int(s[1])
			}
		case []int32:
			if len(s) >= 2 {
				return int(s[0]), int(s[1])
			}
		}
	}
	return 1920, 1080
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case uint32:
		return int(n)
	case int32:
		return int(n)
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}
