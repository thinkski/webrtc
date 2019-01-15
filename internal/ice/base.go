package ice

import (
	"flag"
	"io"
	"log"
	"net"
	"time"
)

var flagEnableIPv6 bool

func init() {
	flag.BoolVar(&flagEnableIPv6, "6", false, "Allow use of IPv6")
}

// [RFC8445] defines a base to be "The transport address that an ICE agent sends from for a
// particular candidate." It is represented here by a UDP connection, listening on a single port.
type Base struct {
	*net.UDPConn
	address   TransportAddress
	component int

	// STUN response handlers for transactions sent from this base, keyed by transaction ID.
	transactions map[string]stunHandler
}

type stunHandler func(msg *stunMessage, addr net.Addr, base Base)

// Create a base for each local IP address.
func establishBases(component int) (bases []Base, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, iface := range ifaces {
		trace("Interface %d: %s (%s)\n", iface.Index, iface.Name, iface.Flags)
		if iface.Flags&net.FlagLoopback != 0 {
			// Skip loopback interfaces to reduce the number of candidates.
			// TODO: Probably we need these if we're not connected to any network.
			continue
		}
		var addrs []net.Addr
		addrs, err = iface.Addrs()
		if err != nil {
			return
		}
		for _, addr := range addrs {
			trace("Local address %v", addr)
			ipnet, ok := addr.(*net.IPNet)
			if !ok {
				log.Panicf("Unexpected address type: %T", addr)
			}

			ip := ipnet.IP
			if !flagEnableIPv6 {
				if ip4 := ip.To4(); ip4 == nil {
					// Must be an IPv6 address. Skip it.
					continue
				}
			}

			base, err := createBase(ip, component)
			if err != nil {
				log.Printf("Failed to create base for %s\n", ip)
				// This can happen for link-local IPv6 addresses. Just skip it.
				continue
			}
			bases = append(bases, base)
		}
	}
	return
}

func createBase(ip net.IP, component int) (base Base, err error) {
	// Listen on an arbitrary UDP port.
	listenAddr := &net.UDPAddr{IP: ip, Port: 0}
	conn, err := net.ListenUDP("udp", listenAddr)
	if err != nil {
		return
	}

	address := makeTransportAddress(conn.LocalAddr())
	log.Printf("Listening on %s\n", address)

	transactions := make(map[string]stunHandler)
	base = Base{conn, address, component, transactions}
	return
}

// Send a STUN message to the given remote address. If a handler is supplied, it will be used to
// process the STUN response, based on the transaction ID.
func (base *Base) sendStun(msg *stunMessage, raddr net.Addr, handler stunHandler) error {
	_, err := base.WriteTo(msg.Bytes(), raddr)
	if err == nil && handler != nil {
		base.transactions[msg.transactionID] = handler
	}
	return err
}

// Read continuously from the connection. STUN messages go to handlers, other data to dataIn.
func (base *Base) demuxStun(defaultHandler stunHandler, dataIn chan<- []byte) {
	buf := make([]byte, 4096)
	for {
		base.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, raddr, err := base.ReadFrom(buf)
		if err == io.EOF {
			log.Printf("Connection closed: %s\n", base.address)
			return
		} else if err != nil {
			if nerr, ok := err.(net.Error); ok {
				if nerr.Timeout() {
					// Timeout is expected for bases that end up not being used.
					log.Printf("Connection timed out: %s\n", base.address)
					return
				}
			}
			log.Fatal(err)
		}
		data := buf[0:n]

		msg, err := parseStunMessage(data)
		if err != nil {
			log.Fatal(err)
		}

		if msg != nil {
			trace("Received from %s: %s\n", raddr, msg)

			// Pass incoming STUN message to the appropriate handler.
			if handler, found := base.transactions[msg.transactionID]; found {
				delete(base.transactions, msg.transactionID)
				handler(msg, raddr, *base)
			} else {
				defaultHandler(msg, raddr, *base)
			}
		} else {
			select {
			case dataIn <- data:
			default:
				//trace("Warning: Data discarded")
			}
		}
	}
}
