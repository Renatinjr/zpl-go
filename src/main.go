package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"

	"github.com/google/gousb"
)

const (
	usbVendorID  = 0x0a5f // Zebra Technologies vendor ID
	usbProductID = 0x00d4 // TLP 2844 product ID
)

type PrinterConnection interface {
	SendZPL(zpl string) error
	Close() error
}

// USBPrinter handles USB-connected Zebra printers
type USBPrinter struct {
	ctx     *gousb.Context
	dev     *gousb.Device
	intf    *gousb.Interface
	outEP   *gousb.OutEndpoint
	claimed bool
}

// NetworkPrinter handles network-connected Zebra printers
type NetworkPrinter struct {
	conn net.Conn
	addr string
}

func main() {
	fmt.Println("Zebra TLP 2844 Printer Control")
	fmt.Println("==============================")

	var printer PrinterConnection
	var err error

	for {
		fmt.Println("\nSelect connection type:")
		fmt.Println("1. USB")
		fmt.Println("2. Network (TCP)")
		fmt.Println("3. Exit")
		fmt.Print("Enter choice: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			printer, err = NewUSBPrinter()
			if err != nil {
				fmt.Printf("USB printer error: %v\n", err)
				continue
			}
			defer printer.Close()
			fmt.Println("Connected to USB printer")
			break
		case "2":
			fmt.Print("Enter printer IP address (e.g., 192.168.1.100:9100): ")
			addr, _ := reader.ReadString('\n')
			addr = strings.TrimSpace(addr)
			printer, err = NewNetworkPrinter(addr)
			if err != nil {
				fmt.Printf("Network printer error: %v\n", err)
				continue
			}
			defer printer.Close()
			fmt.Println("Connected to network printer")
			break
		case "3":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid choice, please try again")
			continue
		}

		if printer != nil {
			break
		}
	}

	for {
		fmt.Println("\nOptions:")
		fmt.Println("1. Send test label")
		fmt.Println("2. Enter custom ZPL")
		fmt.Println("3. Change printer connection")
		fmt.Println("4. Exit")
		fmt.Print("Enter choice: ")

		reader := bufio.NewReader(os.Stdin)
		choice, _ := reader.ReadString('\n')
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			testZPL := `^XA
^FO50,50^A0N,50,50^FDZebra TLP 2844 Test^FS
^FO50,120^A0N,30,30^FDThis is a test label^FS
^FO50,170^BCN,100,Y,N,N^FD123456789^FS
^XZ`
			err := printer.SendZPL(testZPL)
			if err != nil {
				fmt.Printf("Error sending ZPL: %v\n", err)
			} else {
				fmt.Println("Test label sent successfully")
			}
		case "2":
			fmt.Println("Enter ZPL commands (end with blank line):")
			var zplBuilder strings.Builder
			for {
				line, _ := reader.ReadString('\n')
				if strings.TrimSpace(line) == "" {
					break
				}
				zplBuilder.WriteString(line)
			}
			err := printer.SendZPL(zplBuilder.String())
			if err != nil {
				fmt.Printf("Error sending ZPL: %v\n", err)
			} else {
				fmt.Println("ZPL sent successfully")
			}
		case "3":
			printer.Close()
			main() // Restart the program to choose new connection
			return
		case "4":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Println("Invalid choice, please try again")
		}
	}
}

// NewUSBPrinter creates a new USB printer connection
func NewUSBPrinter() (*USBPrinter, error) {
	ctx := gousb.NewContext()
	defer func() {
		if ctx != nil {
			ctx.Close()
		}
	}()

	// Find the Zebra printer
	dev, err := ctx.OpenDeviceWithVIDPID(usbVendorID, usbProductID)
	if err != nil {
		return nil, fmt.Errorf("failed to open device: %v", err)
	}
	if dev == nil {
		return nil, errors.New("Zebra printer not found")
	}

	// On Linux, we need to detach the kernel driver
	if runtime.GOOS == "linux" {
		if err := dev.SetAutoDetach(true); err != nil {
			dev.Close()
			return nil, fmt.Errorf("failed to set auto detach: %v", err)
		}
	}

	// Claim the interface
	intf, done, err := dev.DefaultInterface()
	if err != nil {
		dev.Close()
		return nil, fmt.Errorf("failed to claim interface: %v", err)
	}

	// Find the bulk OUT endpoint
	var outEP *gousb.OutEndpoint
	for _, ep := range intf.Setting.Endpoints {
		if ep.Direction == gousb.EndpointDirectionOut {
			outEP, err = intf.OutEndpoint(ep.Number)
			if err != nil {
				done()
				dev.Close()
				return nil, fmt.Errorf("failed to open out endpoint: %v", err)
			}
			break
		}
	}
	if outEP == nil {
		done()
		dev.Close()
		return nil, errors.New("no OUT endpoint found")
	}

	// Success - don't close the context
	ctx = nil

	return &USBPrinter{
		ctx:     ctx,
		dev:     dev,
		intf:    intf,
		outEP:   outEP,
		claimed: true,
	}, nil
}

// SendZPL sends ZPL commands to the USB printer
func (p *USBPrinter) SendZPL(zpl string) error {
	if !p.claimed {
		return errors.New("printer interface not claimed")
	}

	// Ensure ZPL ends with a newline
	if !strings.HasSuffix(zpl, "\n") {
		zpl += "\n"
	}

	_, err := p.outEP.Write([]byte(zpl))
	return err
}

// Close releases USB resources
func (p *USBPrinter) Close() error {
	var err error
	if p.claimed {
		p.intf.Close()
		p.claimed = false
	}
	if p.dev != nil {
		err = p.dev.Close()
		p.dev = nil
	}
	if p.ctx != nil {
		p.ctx.Close()
		p.ctx = nil
	}
	return err
}

// NewNetworkPrinter creates a new network printer connection
func NewNetworkPrinter(addr string) (*NetworkPrinter, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to printer: %v", err)
	}

	return &NetworkPrinter{
		conn: conn,
		addr: addr,
	}, nil
}

// SendZPL sends ZPL commands to the network printer
func (p *NetworkPrinter) SendZPL(zpl string) error {
	if p.conn == nil {
		return errors.New("not connected to printer")
	}

	// Ensure ZPL ends with a newline
	if !strings.HasSuffix(zpl, "\n") {
		zpl += "\n"
	}

	_, err := io.Copy(p.conn, bytes.NewBufferString(zpl))
	return err
}

// Close closes the network connection
func (p *NetworkPrinter) Close() error {
	if p.conn != nil {
		err := p.conn.Close()
		p.conn = nil
		return err
	}
	return nil
}
