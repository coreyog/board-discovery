/*
 * This file is part of arduino-create-agent.
 *
 * arduino-create-agent is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
 *
 * As a special exception, you may use this file as part of a free software
 * library without restriction.  Specifically, if other files instantiate
 * templates or use macros or inline functions from this file, or you compile
 * this file and link it with other files to produce an executable, this
 * file does not by itself cause the resulting executable to be covered by
 * the GNU General Public License.  This exception does not however
 * invalidate any other reasons why the executable file might be covered by
 * the GNU General Public License.
 *
 * Copyright 2017 ARDUINO AG (http://www.arduino.cc/)
 */

// Package discovery keeps an updated list of the devices connected to the
// computer, via serial ports or found in the network
//
// Usage:
// 	monitor := discovery.New(time.Millisecond)
// 	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
// 	monitor.Start(ctx)
// 	time.Sleep(10 * time.Second)
// 	fmt.Println(monitor.Serial())
// 	fmt.Println(monitor.Network())
//
// Output:
// 	map[/dev/ttyACM0:0x2341/0x8036]
// 	map[192.168.1.107:YunShield]
//
// You may also decide to subscribe to the Events channel of the Monitor:
//
// 	monitor := discovery.New(time.Millisecond)
// 	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
// 	monitor.Start(ctx)
// 	for ev := range monitor.Events {
// 		fmt.Println(ev)
// 	}
//
// Output:
// {add 0x2341/0x8036 <nil>}
// {add <nil> YunShield}
package discovery

import (
	"fmt"
	"time"

	"github.com/codeclysm/cc"
	serial "github.com/facchinm/go-serial-native"
)

// SerialDevice is something connected to the Serial Ports
type SerialDevice struct {
	Port         string       `json:"port"`
	SerialNumber string       `json:"serial_number"`
	ProductID    string       `json:"pid"`
	VendorID     string       `json:"vid"`
	Serial       *serial.Info `json:"-"`
}

func (d SerialDevice) String() string {
	if d.SerialNumber != "" {
		return fmt.Sprintf(`%s/%s/%s`, d.VendorID, d.ProductID, d.SerialNumber)
	}
	return fmt.Sprintf(`%s/%s`, d.VendorID, d.ProductID)
}

//SerialDevices is a list of currently connected devices to the computer
type SerialDevices map[string]*SerialDevice

func (sds SerialDevices) String() string {
	ret := ""
	for _, device := range sds {
		ret += fmt.Sprintln("    ", device)
	}
	return ret
}

// NetworkDevice is something connected to the Network Ports
type NetworkDevice struct {
	Address string `json:"address"`
	Info    string `json:"info"`
	Name    string `json:"name"`
	Port    int    `json:"port"`
}

func (d NetworkDevice) String() string {
	return d.Name
}

//NetworkDevices is a list of currently connected devices to the computer
type NetworkDevices map[string]*NetworkDevice

func (nds NetworkDevices) String() string {
	ret := ""
	for _, device := range nds {
		ret += fmt.Sprintln("    ", device)
	}
	return ret
}

// Event tells you that something has changed in the list of connected devices.
// Name can be one of ["Add", "Change", "Remove"]
// SerialDevice or NetworkDevice can be present and they refer to the device
// that has been added, changed, or removed
type Event struct {
	Name          string         `json:"name"`
	SerialDevice  *SerialDevice  `json:"serial_device,omitempty"`
	NetworkDevice *NetworkDevice `json:"network_device,omitempty"`
}

// Monitor periodically checks the serial ports and the network in order to have
// an updated list of Serial and Network ports.
//
// You can subscribe to the Events channel to get realtime notification of what's changed
type Monitor struct {
	Interval time.Duration
	Events   chan Event

	serial    SerialDevices
	network   NetworkDevices
	stoppable *cc.Stoppable
}

// New Creates a new monitor that can start querying the serial ports and
// the local network for devices
func New(interval time.Duration) *Monitor {
	m := Monitor{
		serial:   SerialDevices{},
		network:  NetworkDevices{},
		Interval: interval,
	}
	return &m
}

// Start begins the loop that queries the serial ports and the local network.
// It accepts a cancelable context
func (m *Monitor) Start() {
	cc.Run(func(stopSignal chan struct{}) {
		m.Events = make(chan (Event))
		var done chan bool
		var stop = false

		go func() {
			<-stopSignal
			stop = true
		}()

		go func() {
			for {
				if stop {
					break
				}
				m.serialDiscover()
			}
			done <- true
		}()
		go func() {
			for {
				if stop {
					break
				}
				m.networkDiscover()
			}
			done <- true
		}()

		go func() {
			// We need to wait until both goroutines have finished
			<-done
			<-done
			close(m.Events)
		}()
	})
}

func (m *Monitor) Stop() {
	if m.stoppable != nil {
		m.stoppable.Stop()
		<-m.stoppable.Stopped
		m.stoppable = nil
	}
}

func (m *Monitor) Restart() {
	m.Stop()
	m.Start()
}

// Serial returns a cached list of devices connected to the serial ports
func (m *Monitor) Serial() SerialDevices {
	return m.serial
}

// Network returns a cached list of devices found on the local network
func (m *Monitor) Network() NetworkDevices {
	return m.network
}

func (m *Monitor) String() string {
	return fmt.Sprintln("DEVICES:") +
		fmt.Sprintln("  SERIAL:") +
		fmt.Sprintln(m.serial) +
		fmt.Sprintln("  NETWORK:") +
		fmt.Sprintln(m.network)
}
