package network

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Potsdam-Sensors/GoLinuxToolkit/unix"
	"github.com/godbus/dbus/v5"
)

const (
	MethodDbusGetProperty  = "org.freedesktop.DBus.Properties.Get"
	MethodDbusAddMatchRule = "org.freedesktop.DBus.AddMatch"

	SystemdInterface  = "org.freedesktop.systemd1"
	SystemdObjectPath = dbus.ObjectPath("/org/freedesktop/systemd1")

	NetworkManagerInterface                = "org.freedesktop.NetworkManager"
	NetworkManagerDeviceInterface          = "org.freedesktop.NetworkManager.Device"
	NetworkManagerObjectPath               = dbus.ObjectPath("/org/freedesktop/NetworkManager")
	NetworkManagerSignalState              = "StateChanged"
	NetworkManagerMethodGetState           = "org.freedesktop.NetworkManager.state"
	NetworkManagerMethodCheckConnectivity  = "org.freedesktop.NetworkManager.CheckConnectivity"
	NetworkManagerMethodGetDeviceFromIFace = "org.freedesktop.NetworkManager.GetDeviceByIpIface"
	NetworkManagerMethodWirelessSSIDScan   = "org.freedesktop.NetworkManager.Device.Wireless.RequestScan"
	NetworkManagerMethodGetSSIDs           = "org.freedesktop.NetworkManager.Device.Wireless.GetAccessPoints"
)

const (
	DeviceStateChangedSignal = NetworkManagerDeviceInterface + ".StateChanged"
)

const (
	NM_STATE_UNKNOWN          = 0  // networking state is unknown
	NM_STATE_ASLEEP           = 10 // networking is not enabled
	NM_STATE_DISCONNECTED     = 20 // there is no active network connection
	NM_STATE_DISCONNECTING    = 30 // network connections are being cleaned up
	NM_STATE_CONNECTING       = 40 // a network connection is being started
	NM_STATE_CONNECTED_LOCAL  = 50 // there is only local IPv4 and/or IPv6 connectivity
	NM_STATE_CONNECTED_SITE   = 60 // there is only site-wide IPv4 and/or IPv6 connectivity
	NM_STATE_CONNECTED_GLOBAL = 70 // there is global IPv4 and/or IPv6 Internet connectivity
)

var NM_STATE_MAP = map[uint32]string{
	NM_STATE_UNKNOWN:          "Unknown",
	NM_STATE_ASLEEP:           "Asleep",
	NM_STATE_DISCONNECTED:     "Disconnected",
	NM_STATE_DISCONNECTING:    "Disconnecting",
	NM_STATE_CONNECTING:       "Connecting",
	NM_STATE_CONNECTED_LOCAL:  "Connected - Local",
	NM_STATE_CONNECTED_SITE:   "Connected - Site",
	NM_STATE_CONNECTED_GLOBAL: "Connected - Global",
}

const (
	NM_CONNECTIVITY_UNKNOWN = 0 // Network connectivity is unknown.
	NM_CONNECTIVITY_NONE    = 1 // The host is not connected to any network.
	NM_CONNECTIVITY_PORTAL  = 2 // The host is behind a captive portal and cannot reach the full Internet.
	NM_CONNECTIVITY_LIMITED = 3 // The host is connected to a network, but does not appear to be able to reach the full Internet.
	NM_CONNECTIVITY_FULL    = 4 // The host is connected to a network, and appears to be able to reach the full Internet.
)

var NM_CONNECTIVITY_MAP = map[uint32]string{
	NM_CONNECTIVITY_UNKNOWN: "Unknown",
	NM_CONNECTIVITY_NONE:    "None",
	NM_CONNECTIVITY_PORTAL:  "Portal",
	NM_CONNECTIVITY_LIMITED: "Limited",
	NM_CONNECTIVITY_FULL:    "Full",
}

const (
	NM_DEVICE_STATE_UNKNOWN      = 0   // the device's state is unknown
	NM_DEVICE_STATE_UNMANAGED    = 10  // the device is recognized, but not managed by NetworkManager
	NM_DEVICE_STATE_UNAVAILABLE  = 20  // the device is managed by NetworkManager, but is not available for use. Reasons may include the wireless switched off, missing firmware, no ethernet carrier, missing supplicant or modem manager, etc.
	NM_DEVICE_STATE_DISCONNECTED = 30  // the device can be activated, but is currently idle and not connected to a network.
	NM_DEVICE_STATE_PREPARE      = 40  // the device is preparing the connection to the network. This may include operations like changing the MAC address, setting physical link properties, and anything else required to connect to the requested network.
	NM_DEVICE_STATE_CONFIG       = 50  // the device is connecting to the requested network. This may include operations like associating with the WiFi AP, dialing the modem, connecting to the remote Bluetooth device, etc.
	NM_DEVICE_STATE_NEED_AUTH    = 60  // the device requires more information to continue connecting to the requested network. This includes secrets like WiFi passphrases, login passwords, PIN codes, etc.
	NM_DEVICE_STATE_IP_CONFIG    = 70  // the device is requesting IPv4 and/or IPv6 addresses and routing information from the network.
	NM_DEVICE_STATE_IP_CHECK     = 80  // the device is checking whether further action is required for the requested network connection. This may include checking whether only local network access is available, whether a captive portal is blocking access to the Internet, etc.
	NM_DEVICE_STATE_SECONDARIES  = 90  // the device is waiting for a secondary connection (like a VPN) which must activated before the device can be activated
	NM_DEVICE_STATE_ACTIVATED    = 100 // the device has a network connection, either local or global.
	NM_DEVICE_STATE_DEACTIVATING = 110 // a disconnection from the current network connection was requested, and the device is cleaning up resources used for that connection. The network connection may still be valid.
	NM_DEVICE_STATE_FAILED       = 120 // the device failed to connect to the requested network and is cleaning up the connection request*/
)

var NM_DEVICE_STATE_MAP = map[uint32]string{
	NM_DEVICE_STATE_UNKNOWN:      "Unknown",
	NM_DEVICE_STATE_UNMANAGED:    "Unmanaged",
	NM_DEVICE_STATE_UNAVAILABLE:  "Unavailable",
	NM_DEVICE_STATE_DISCONNECTED: "Disconnected",
	NM_DEVICE_STATE_PREPARE:      "Prepare",
	NM_DEVICE_STATE_CONFIG:       "Config",
	NM_DEVICE_STATE_NEED_AUTH:    "Need Auth",
	NM_DEVICE_STATE_IP_CONFIG:    "IP Config",
	NM_DEVICE_STATE_IP_CHECK:     "IP Check",
	NM_DEVICE_STATE_SECONDARIES:  "Secondaries",
	NM_DEVICE_STATE_ACTIVATED:    "Activated",
	NM_DEVICE_STATE_DEACTIVATING: "Deactivating",
	NM_DEVICE_STATE_FAILED:       "Failed",
}

func GetNetworkManagerStateSubscription() (*unix.DBusSignalSubscription, error) {
	matchRule := fmt.Sprintf("type='signal',interface='%s',member='%s',path='%s'", unix.NetworkManagerInterface, unix.NetworkManagerSignalState, unix.NetworkManagerObjectPath)
	sub := &unix.DBusSignalSubscription{}
	err := sub.MakeDBusSignalSubscription(matchRule, 20)
	return sub, err
}

func getNetworkManagerObject(conn *dbus.Conn) *dbus.BusObject {
	nm := conn.Object(NetworkManagerInterface, NetworkManagerObjectPath)
	return &nm
}
func GetNetworkManagerState(conn *dbus.Conn) (uint32, error) {
	nmObj := getNetworkManagerObject(conn)
	if nmObj == nil {
		return 0, errors.New("failed to retrieve NetworkManager object")
	}
	call := (*nmObj).Call(NetworkManagerMethodGetState, 0)
	if call.Err != nil {
		return 0, fmt.Errorf("error calling %s: %v", NetworkManagerMethodGetState, call.Err)
	}
	var state uint32
	err := call.Store(&state)
	if err != nil {
		return 0, fmt.Errorf("error storing state from call: %v", err)
	}
	return state, nil
}

func GetNetworkManagerConnectivity(conn *dbus.Conn) (uint32, error) {
	nmObj := getNetworkManagerObject(conn)
	if nmObj == nil {
		return 0, errors.New("failed to retrieve NetworkManager object")
	}
	call := (*nmObj).Call(NetworkManagerMethodCheckConnectivity, 0)
	if call.Err != nil {
		return 0, fmt.Errorf("error from call to %s: %v", NetworkManagerMethodCheckConnectivity, call.Err)
	}
	var state uint32
	err := call.Store(&state)
	if err != nil {
		return 0, fmt.Errorf("error storing result from call: %v", err)
	}
	return state, nil
}

func getDevicesFromConnection(connObj *dbus.BusObject) ([]dbus.ObjectPath, error) {
	connActiveInterface := "org.freedesktop.NetworkManager.Connection.Active"
	var devicePaths []dbus.ObjectPath
	variant, err := (*connObj).GetProperty(connActiveInterface + ".Devices")
	if err != nil {
		return nil, fmt.Errorf("error during property read %s: %v", connActiveInterface+".Devices", err)
	}
	err = variant.Store(&devicePaths)
	if err != nil {
		return nil, fmt.Errorf("error storing variant: %v", err)
	}
	return devicePaths, nil
}

func GetPrimaryDevicePath(conn *dbus.Conn) (dbus.ObjectPath, error) {
	nmObj := getNetworkManagerObject(conn)
	if nmObj == nil {
		return "", errors.New("failed to retrieve NetworkManager object")
	}

	// Get ObjectPath of the primary connection
	var connPath dbus.ObjectPath
	call := (*nmObj).Call(MethodDbusGetProperty, 0, NetworkManagerInterface, "PrimaryConnection")
	if call.Err != nil {
		return "", fmt.Errorf("error calling %s: %v", MethodDbusGetProperty, call.Err)
	}
	err := call.Store(&connPath)
	if err != nil {
		return "", fmt.Errorf("error storing result of call: %v", err)
	}

	// Get the device from the connection object
	connObj := conn.Object(NetworkManagerInterface, connPath)
	if connObj == nil {
		return "", fmt.Errorf("failed to get connection object")
	}
	//

	// Get the Devices property
	devicePaths, err := getDevicesFromConnection(&connObj)
	if err != nil {
		return "", err
	}

	if len(devicePaths) > 1 {
		log.Printf("[Warning] More than one device path for primary connection.")
	} else if len(devicePaths) == 0 {
		return "", errors.New("no devices are associated with the primary connection")
	}
	return devicePaths[0], nil
}

func GetDeviceObjectFromPath(conn *dbus.Conn, devPath dbus.ObjectPath) (*dbus.BusObject, error) {
	nmObj := getNetworkManagerObject(conn)
	if nmObj == nil {
		return nil, errors.New("failed to retrieve NetworkManager object")
	}
	device := conn.Object(unix.NetworkManagerInterface, devPath)
	if device == nil {
		return nil, fmt.Errorf("failed to retrieve object at %s", devPath)
	}
	return &device, nil
}

func GetDeviceInterfaceName(conn *dbus.Conn, devObj *dbus.BusObject) (string, error) {
	variant, err := (*devObj).GetProperty(NetworkManagerInterface + ".Device.Interface")
	if err != nil {
		return "", fmt.Errorf("failed to read property of device: %v", err)
	}
	var interfaceName string
	err = variant.Store(&interfaceName)
	if err != nil {
		return "", fmt.Errorf("error storing data: %v", err)
	}
	return interfaceName, nil
}

func GetPrimaryDeviceObject(conn *dbus.Conn) (*dbus.BusObject, error) {
	devPath, err := GetPrimaryDevicePath(conn)
	if err != nil {
		return nil, err
	}
	return GetDeviceObjectFromPath(conn, devPath)
}

func GetDevicePathFromInterfaceName(conn *dbus.Conn, interfaceName string) (dbus.ObjectPath, error) {
	nmObj := getNetworkManagerObject(conn)
	if nmObj == nil {
		return "", errors.New("failed to retrieve NetworkManager object")
	}
	call := (*nmObj).Call(NetworkManagerMethodGetDeviceFromIFace, 0, interfaceName)
	if call.Err != nil {
		return "", fmt.Errorf("error during call %s: %v", NetworkManagerMethodGetDeviceFromIFace, call.Err)
	}

	var devicePath dbus.ObjectPath
	err := call.Store(&devicePath)
	if err != nil {
		return "", fmt.Errorf("error storing value from call: %v", err)
	}
	return devicePath, nil
}

type SSIDInfo struct {
	SSID       []byte
	ObjectPath dbus.ObjectPath
}

// GetAvailableSSIDs returns a list of available SSIDs and their D-Bus paths.
func GetAvailableSSIDs(conn *dbus.Conn, devObj *dbus.BusObject) ([]SSIDInfo, error) {
	call := (*devObj).Call(NetworkManagerMethodWirelessSSIDScan, 0, map[string]dbus.Variant{})
	if call.Err != nil {
		return nil, fmt.Errorf("error in call to %s: %v", NetworkManagerMethodWirelessSSIDScan, call.Err)
	}
	err := call.Store() // I think this is to make sure execution happens?
	if err != nil {
		return nil, fmt.Errorf("error storing call: %v", err)
	}

	time.Sleep(time.Second)

	call = (*devObj).Call(NetworkManagerMethodGetSSIDs, 0)
	if call.Err != nil {
		return nil, fmt.Errorf("error in call to %s: %v", NetworkManagerMethodGetSSIDs, call.Err)
	}
	var ssids []dbus.ObjectPath
	err = call.Store(&ssids)
	if err != nil {
		return nil, fmt.Errorf("error storing call: %v", err)
	}
	ssidInfos := make([]SSIDInfo, len(ssids))
	for i, ap := range ssids {
		var ssid []byte
		err = conn.Object(NetworkManagerInterface, ap).Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.NetworkManager.AccessPoint", "Ssid").Store(&ssid)
		if err != nil {
			log.Printf("[Warning] Error getting SSID Info: %v", err)
			continue
		}
		ssidInfos[i] = SSIDInfo{
			SSID:       ssid,
			ObjectPath: ap,
		}
	}

	return ssidInfos, nil
}

func GetDeviceFromInterfaceName(conn *dbus.Conn, interfaceName string) (*dbus.BusObject, error) {
	devPath, err := GetDevicePathFromInterfaceName(conn, interfaceName)
	if err != nil {
		return nil, err
	}
	return GetDeviceObjectFromPath(conn, devPath)
}

func CheckDeviceState(conn *dbus.Conn, devObj *dbus.BusObject) (uint32, error) {
	var state uint32
	err := (*devObj).Call(MethodDbusGetProperty, 0, "org.freedesktop.NetworkManager.Device", "State").Store(&state)
	return state, err
}

func getConnectionSettings(ssid string, pass string) map[string]map[string]dbus.Variant {
	return map[string]map[string]dbus.Variant{
		"802-11-wireless": {
			"ssid": dbus.MakeVariant([]byte(ssid)), // SSID needs to be a byte slice
		},
		"802-11-wireless-security": {
			"key-mgmt": dbus.MakeVariant("wpa-psk"),
			"psk":      dbus.MakeVariant(pass),
		},
		"connection": {
			"id":          dbus.MakeVariant(ssid),
			"type":        dbus.MakeVariant("802-11-wireless"),
			"autoconnect": dbus.MakeVariant(true),
		},
		"ipv4": {
			"method": dbus.MakeVariant("auto"),
		},
		"ipv6": {
			"method": dbus.MakeVariant("auto"),
		},
	}
}
func ConnectToSSID(ssid string, pass string, conn *dbus.Conn, devPath dbus.ObjectPath) error {
	// TODO Clean this up
	devObj, err := GetDeviceObjectFromPath(conn, devPath)
	if err != nil {
		return err
	}

	ssids, err := GetAvailableSSIDs(conn, devObj)
	if err != nil {
		return fmt.Errorf("failed to scan SSIDS: %w", err)
	}

	var ssidPath dbus.ObjectPath
	ssidMatched := false
	for _, si := range ssids {
		if string(si.SSID) == ssid {
			ssidMatched = true
			ssidPath = si.ObjectPath
			break
		}
	}
	if !ssidMatched {
		return fmt.Errorf("failed to find SSID matching given \"%s\"", ssid)
	}

	connectionSettings := getConnectionSettings(ssid, pass)
	var (
		activeConnectionPath dbus.ObjectPath
		devicePath           dbus.ObjectPath
	)

	err = conn.Object(NetworkManagerInterface, NetworkManagerObjectPath).Call(
		"org.freedesktop.NetworkManager.AddAndActivateConnection", 0,
		connectionSettings, devPath, ssidPath,
	).Store(&activeConnectionPath, &devicePath)
	if err != nil {
		return fmt.Errorf("failed to add and activate connection: %w", err)
	}
	fmt.Printf("Connection activated: %s on device: %s\n", activeConnectionPath, devicePath)
	return nil
}

/*
C <- (new state, old state, reason)
*/
type DeviceStateChangeSubscription struct {
	C    chan [3]uint32
	Stop func()
	Join func()
}

func deviceStateChangeSubscribe(devPath dbus.ObjectPath) (*dbus.Conn, chan *dbus.Signal, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to System Bus: %w", err)
	}

	matchRule := dbus.WithMatchObjectPath(devPath)
	conn.AddMatchSignal(matchRule)
	c := make(chan *dbus.Signal, 20)
	conn.Signal(c)

	return conn, c, nil
}

func goParseDeviceStateChangeSignals(ctx context.Context, wg *sync.WaitGroup, conn *dbus.Conn, devPath dbus.ObjectPath, sigCh chan *dbus.Signal, outCh chan [3]uint32) {
	defer wg.Done()
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-sigCh:
			if (sig.Path == devPath) && (sig.Name == DeviceStateChangedSignal) && (len(sig.Body) >= 3) {
				var values [3]uint32
				v, ok := sig.Body[0].(uint32)
				if !ok {
					continue
				}
				values[0] = v
				v, ok = sig.Body[1].(uint32)
				if !ok {
					continue
				}
				values[1] = v
				v, ok = sig.Body[2].(uint32)
				if !ok {
					continue
				}
				values[2] = v
				outCh <- values
			}
		}
	}

}

func DeviceStateChangeSubscribe(devPath dbus.ObjectPath) (*DeviceStateChangeSubscription, error) {
	conn, sigCh, err := deviceStateChangeSubscribe(devPath)
	if err != nil {
		return nil, err
	}
	outCh := make(chan [3]uint32, 20)
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go goParseDeviceStateChangeSignals(ctx, wg, conn, devPath, sigCh, outCh)
	ret := &DeviceStateChangeSubscription{
		C:    outCh,
		Stop: cancel,
		Join: wg.Wait,
	}
	return ret, nil
}
