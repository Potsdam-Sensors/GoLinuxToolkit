package systemd

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/godbus/dbus"
)

const (
	systemdService           = "org.freedesktop.systemd1"
	systemObjectPath         = dbus.ObjectPath("/org/freedesktop/systemd1")
	systemdGetUnitMethod     = "org.freedesktop.systemd1.Manager.GetUnit"
	dbusGetPropertyMethod    = "org.freedesktop.DBus.Properties.Get"
	systemdUnit              = "org.freedesktop.systemd1.Unit"
	systemdUnitStateProperty = "ActiveState"
	systemdStopUnitMethod    = "org.freedesktop.systemd1.Manager.StopUnit"
	systemdStartUnitMethod   = "org.freedesktop.systemd1.Manager.StartUnit"

	systemdJobRemovedMatchRule = "type='signal',interface='org.freedesktop.systemd1.Manager',member='JobRemoved'"
	dbusAddMatchRuleMethod     = "org.freedesktop.DBus.AddMatch"
	dbusJobRemovedSignalName   = "org.freedesktop.systemd1.Manager.JobRemoved"
)

func getSystemdObject(conn *dbus.Conn) (*dbus.BusObject, error) {
	systemdObj := conn.Object(systemdService, systemObjectPath)
	if systemdObj == nil {
		return nil, fmt.Errorf("failed to get systemd object")
	}
	return &systemdObj, nil
}

func getSystemdUnitObject(conn *dbus.Conn, serviceName string) (*dbus.BusObject, error) {
	systemdObj, err := getSystemdObject(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd obj: %v", err)
	}
	var unitObjectPath dbus.ObjectPath
	call := (*systemdObj).Call(systemdGetUnitMethod, 0, serviceName)
	//The name org.freedesktop.systemdl was not provided by any .service files
	if call.Err != nil {
		return nil, fmt.Errorf("failed to get unit path %s: %v", serviceName, call.Err)
	}
	call.Store(&unitObjectPath)

	unitObj := conn.Object(systemdService, unitObjectPath)
	if unitObj == nil {
		return nil, fmt.Errorf("failed to get unit object")
	}
	return &unitObj, nil
}

func getUnitStatus(unitObj *dbus.BusObject) (string, error) {
	var state string
	call := (*unitObj).Call(dbusGetPropertyMethod, 0, systemdUnit, systemdUnitStateProperty)
	if call.Err != nil {
		return "", fmt.Errorf("failed to check unit state: %v", call.Err)
	}
	call.Store(&state)
	return state, nil
}

func checkServiceStatus(conn *dbus.Conn, serviceName string) (*dbus.BusObject, bool, error) {
	unitObj, err := getSystemdUnitObject(conn, serviceName)
	if err != nil {
		return nil, false, err
	}

	unitState, err := getUnitStatus(unitObj)
	if err != nil {
		return nil, false, err
	}
	log.Printf("Service %s has unit state: %s", serviceName, unitState)
	return unitObj, !((unitState == "inactive") || (unitState == "failed")), nil
}

func CheckServiceStatus(serviceName string) (bool, error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return false, fmt.Errorf("failed to connected to the system bus: %v", err)
	}
	defer conn.Close()

	_, res, err := checkServiceStatus(conn, serviceName)
	return res, err
}

func doStopService(systemdObj *dbus.BusObject, serviceName string) (dbus.ObjectPath, error) {
	// TODO: I bet this job object is useful for waiting for completion of itself
	var jobObjectPath dbus.ObjectPath
	call := (*systemdObj).Call(systemdStopUnitMethod, 0, serviceName, "replace")
	if call.Err != nil {
		return "", fmt.Errorf("failed to stop unit: %v", call.Err)
	}
	call.Store(&jobObjectPath)
	return jobObjectPath, nil
}

func doStartService(systemdObj *dbus.BusObject, serviceName string) (dbus.ObjectPath, error) {
	// TODO: I bet this job object is useful for waiting for completion of itself
	var jobObjectPath dbus.ObjectPath
	call := (*systemdObj).Call(systemdStartUnitMethod, 0, serviceName, "replace")
	if call.Err != nil {
		return "", fmt.Errorf("failed to start unit: %v", call.Err)
	}
	call.Store(&jobObjectPath)
	return jobObjectPath, nil
}

func waitJobComplete(conn *dbus.Conn, targetJobPath dbus.ObjectPath) (string, error) {
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, systemdJobRemovedMatchRule)
	signalCh := make(chan *dbus.Signal, 10)
	conn.Signal(signalCh)

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			return "", errors.New("operation timed out")
		case signal := <-signalCh:
			if signal.Name == "org.freedesktop.systemd1.Manager.JobRemoved" {
				// Extract data from the signal
				//jobPath, unitName, jobResult
				if len(signal.Body) < 4 {
					log.Printf("[Warning] expected length of job signal body to be at least 4: %v", signal.Body)
					continue
				}
				// jobNum, jobPath, serviceName, jobResult := signal.Body[0], signal.Body[1], signal.Body[2], signal.Body[3]
				jobPath := signal.Body[1]
				jobResult := signal.Body[3]
				if jobPath == targetJobPath {
					switch jobResult := jobResult.(type) {
					case string:
						return jobResult, nil
					default:
						return "", fmt.Errorf("unexpected jobResult type, got value: %v", jobResult)
					}
				}
			}
		}
	}
}

func StartService(serviceName string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connected to the system bus: %v", err)
	}
	defer conn.Close()
	systemdObj, err := getSystemdObject(conn)
	if err != nil {
		return fmt.Errorf("failed to get systemd obj: %v", err)
	}
	_, res, err := checkServiceStatus(conn, serviceName)
	if err != nil {
		return err
	}
	if res {
		log.Printf("Unit %s is already running.", serviceName)
		return nil
	}
	startJobPath, err := doStartService(systemdObj, serviceName)
	if err != nil {
		return fmt.Errorf("error requesting start job for service: %v", err)
	}

	jobResult, err := waitJobComplete(conn, startJobPath)
	if err != nil {
		return fmt.Errorf("waiting for start job failed with error: %v", err)
	}
	log.Printf("Job to start service %s completed with result: %s", serviceName, jobResult)
	if jobResult == "done" {
		return nil
	}
	_, res, err = checkServiceStatus(conn, serviceName)
	if err != nil {
		return fmt.Errorf("job to start unit failed and checking state of service gave error: %v", err)
	} else if !res {
		return fmt.Errorf("job to start service failed (%s) and unit isn't running", jobResult)
	}
	return nil
}

func StopService(serviceName string) error {
	conn, err := dbus.SystemBus()
	if err != nil {
		return fmt.Errorf("failed to connected to the system bus: %v", err)
	}
	defer conn.Close()
	systemdObj, err := getSystemdObject(conn)
	if err != nil {
		return fmt.Errorf("failed to get systemd obj: %v", err)
	}
	_, res, err := checkServiceStatus(conn, serviceName)
	if err != nil {
		return err
	}
	if !res {
		log.Printf("Unit %s is already stopped.", serviceName)
		return nil
	}
	stopJobPath, err := doStopService(systemdObj, serviceName)
	if err != nil {
		return fmt.Errorf("error requesting stop job for service: %v", err)
	}

	jobResult, err := waitJobComplete(conn, stopJobPath)
	if err != nil {
		return fmt.Errorf("waiting for stop job failed with error: %v", err)
	}
	log.Printf("Job to stop service %s completed with result: %s", serviceName, jobResult)
	if jobResult == "done" {
		return nil
	}
	_, res, err = checkServiceStatus(conn, serviceName)
	if err != nil {
		return fmt.Errorf("job to stop unit failed and checking state of service gave error: %v", err)
	} else if res {
		return fmt.Errorf("job to stop service failed (%s) and unit is still running", jobResult)
	}
	return nil
}
