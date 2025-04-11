package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/tarm/serial"
)

func main() {
	// Command line arguments
	portName := flag.String("port", "", "Serial port name (e.g., COM3 on Windows, /dev/ttyUSB0 on Linux)")
	baudRate := flag.Int("baud", 9600, "Baud rate (default: 9600)")
	zplFile := flag.String("file", "", "ZPL file to send (optional)")
	zplText := flag.String("zpl", "", "ZPL command text (optional)")
	flag.Parse()

	// Validate port name
	if *portName == "" {
		log.Fatal("Port name is required. Use -port flag.")
	}

	// Configure serial port
	config := &serial.Config{
		Name:     *portName,
		Baud:     *baudRate,
		Size:     8,
		Parity:   serial.ParityNone,
		StopBits: serial.Stop1,
	}

	fmt.Printf("Connecting to Zebra printer on %s at %d baud...\n", *portName, *baudRate)

	// Open serial port
	port, err := serial.OpenPort(config)
	if err != nil {
		log.Fatalf("Failed to open serial port: %v", err)
	}
	defer port.Close()

	fmt.Println("Connection established.")

	// Handle ZPL commands
	if *zplFile != "" {
		// Send ZPL from file
		fmt.Printf("Sending ZPL from file: %s\n", *zplFile)
		err := sendZplFromFile(port, *zplFile)
		if err != nil {
			log.Fatalf("Failed to send ZPL file: %v", err)
		}
	} else if *zplText != "" {
		// Send ZPL from command line
		fmt.Println("Sending ZPL command...")
		err := sendZpl(port, *zplText)
		if err != nil {
			log.Fatalf("Failed to send ZPL command: %v", err)
		}
	} else {
		// Interactive mode
		fmt.Println("Starting interactive mode. Enter ZPL commands or type 'exit' to quit:")

		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ")
			if !scanner.Scan() {
				break
			}

			input := scanner.Text()
			if strings.ToLower(input) == "exit" {
				break
			}

			if input != "" {
				err := sendZpl(port, input)
				if err != nil {
					fmt.Printf("Error sending command: %v\n", err)
				} else {
					fmt.Println("Command sent successfully.")
				}
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Fprintln(os.Stderr, "Error reading input:", err)
		}
	}

	fmt.Println("Connection closed.")
}

func sendZpl(port io.Writer, zpl string) error {
	// Make sure the command ends with a newline
	if !strings.HasSuffix(zpl, "\n") {
		zpl += "\n"
	}

	_, err := port.Write([]byte(zpl))
	return err
}

func sendZplFromFile(port io.Writer, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read file: %v", err)
	}

	_, err = port.Write(data)
	if err != nil {
		return fmt.Errorf("failed to send data: %v", err)
	}

	return nil
}
