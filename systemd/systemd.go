package systemd

/*
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
}*/
