package main

import (
	"log"

	network "github.com/Potsdam-Sensors/GoLinuxToolkit/network"
	"github.com/godbus/dbus/v5"
)

const (
	wifiInterfaceName = "wlan0"
)

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatalf("Failed to start connection with system bus: %v", err)
	}
	defer conn.Close()
	state, err := network.GetNetworkManagerState(conn)
	log.Printf("State: %d, Err: %v", state, err)

	connectivity, err := network.GetNetworkManagerConnectivity(conn)
	log.Printf("Connectivity: %d, Err: %v", connectivity, err)

	devObj, err := network.GetPrimaryDeviceObject(conn)
	if err != nil {
		log.Fatalf("network.GetPrimaryDeviceObject: %v", err)
	}
	ifName, err := network.GetDeviceInterfaceName(conn, devObj)
	log.Printf("Primary Interface Name: %s, Err: %v", ifName, err)

	wifiObj, err := network.GetDeviceFromInterfaceName(conn, wifiInterfaceName)
	if err != nil {
		log.Fatalf("error getting wifi obj: %v", err)
	}

	ssidInfos, err := network.GetAvailableSSIDs(conn, wifiObj)
	if err != nil {
		log.Fatalf("Error getting SSID infos: %v", err)
	}

	for _, info := range ssidInfos {
		log.Printf("-> %s", info.SSID)
	}

	state, err = network.CheckDeviceState(conn, devObj)
	log.Printf("Device state: %d, Err: %v", state, err)
}
