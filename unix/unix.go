package unix

import (
	"fmt"

	dbus "github.com/godbus/dbus/v5"
)

const (
	MethodDbusGetProperty  = "org.freedesktop.DBus.Properties.Get"
	MethodDbusAddMatchRule = "org.freedesktop.DBus.AddMatch"

	SystemdInterface  = "org.freedesktop.systemd1"
	SystemdObjectPath = dbus.ObjectPath("/org/freedesktop/systemd1")

	NetworkManagerInterface               = "org.freedesktop.NetworkManager"
	NetworkManagerObjectPath              = dbus.ObjectPath("/org/freedesktop/NetworkManager")
	NetworkManagerSignalState             = "StateChanged"
	NetworkManagerMethodGetState          = "org.freedesktop.NetworkManager.state"
	NetworkManagerMethodCheckConnectivity = "org.freedesktop.NetworkManager.CheckConnectivity"

	/*
			ActiveConnections        readable   ao
		PrimaryConnection        readable   o
		PrimaryConnectionType
	*/

	//

	systemdGetUnitMethod = "org.freedesktop.systemd1.Manager.GetUnit"

	systemdUnit              = "org.freedesktop.systemd1.Unit"
	systemdUnitStateProperty = "ActiveState"
	systemdStopUnitMethod    = "org.freedesktop.systemd1.Manager.StopUnit"
	systemdStartUnitMethod   = "org.freedesktop.systemd1.Manager.StartUnit"

	systemdJobRemovedMatchRule = "type='signal',interface='org.freedesktop.systemd1.Manager',member='JobRemoved'"

	dbusJobRemovedSignalName = "org.freedesktop.systemd1.Manager.JobRemoved"
)

/*
You must defer Conn.Close()
*/
type DBusSignalSubscription struct {
	C    chan *dbus.Signal
	Conn *dbus.Conn
}

func (ss *DBusSignalSubscription) MakeDBusSignalSubscription(matchRule string, size int) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connect to SystemBus: %v", err)
	}
	call := conn.BusObject().Call(MethodDbusAddMatchRule, 0, matchRule)
	if call.Err != nil {
		return call.Err
	}

	ch := make(chan *dbus.Signal, size)
	conn.Signal(ch)
	ss.Conn = conn
	ss.C = ch
	return nil
}

func ToDBusObjectPath(str string) dbus.ObjectPath {
	return dbus.ObjectPath(str)
}

func GetDBusConn() *dbus.Conn {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil
	}
	return conn
}

type DBusObjectPath dbus.ObjectPath
type DBusObject dbus.Object
type DBusConn dbus.Conn
