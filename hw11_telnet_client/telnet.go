package main

import (
	"io"
	"net"
	"time"
)

// TelnetClient represents a bidirectional TCP client.
type TelnetClient interface {
	Connect() error
	io.Closer
	Send() error
	Receive() error
}

type telnetClient struct {
	address    string
	timeout    time.Duration
	input      io.ReadCloser
	output     io.Writer
	connection net.Conn
}

// NewTelnetClient creates a client that transfers data between input, output, and address.
func NewTelnetClient(address string, timeout time.Duration, input io.ReadCloser, output io.Writer) TelnetClient {
	return &telnetClient{address: address, timeout: timeout, input: input, output: output}
}

func (c *telnetClient) Connect() error {
	connection, err := net.DialTimeout("tcp", c.address, c.timeout)
	if err != nil {
		return err
	}
	c.connection = connection
	return nil
}

func (c *telnetClient) Send() error {
	_, err := io.Copy(c.connection, c.input)
	return err
}

func (c *telnetClient) Receive() error {
	_, err := io.Copy(c.output, c.connection)
	return err
}

func (c *telnetClient) Close() error {
	inputErr := c.input.Close()
	connectionErr := c.connection.Close()
	if inputErr != nil {
		return inputErr
	}
	return connectionErr
}
