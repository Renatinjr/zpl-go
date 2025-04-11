package main

import (
	"fmt"
	"log"

	"go.bug.st/serial"
)

func main() {
	// Use "USB001" as the port name
	portName := "USB001"

	// Set up serial port mode
	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	// Open the port
	port, err := serial.Open(portName, mode)
	if err != nil {
		log.Fatalf("Failed to open port: %v", err)
	}
	defer port.Close()

	// Simple ZPL label
	zpl := `^XA
^FO20,20^A0N,30,30^FDHello from Go!^FS
^FO20,60^BY2^BCN,60,Y,N,N^FD123456789^FS
^XZ
`

	// Send to printer
	_, err = port.Write([]byte(zpl))
	if err != nil {
		log.Fatalf("Failed to send ZPL: %v", err)
	}

	fmt.Println("Label sent successfully to USB001")
}
