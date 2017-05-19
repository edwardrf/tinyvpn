package main

import (
	"os/exec"
	"strconv"
	"strings"

	"github.com/songgao/water"
	"github.com/songgao/water/waterutil"
)

type Tunnel struct {
	Addr    string
	Mtu     int
	Book    Book
	tun     *water.Interface
	oldGw   string
	oldIfce string
}

func (tunnel *Tunnel) Start() (chan<- *Packet, <-chan *Packet, error) {

	t, err := water.NewTUN("")
	if err != nil {
		return nil, nil, err
	}

	// Add IP address to tun interface
	_, err = exec.Command("ip", "addr", "add", tunnel.Addr, "dev", t.Name()).Output()
	if err != nil {
		return nil, nil, err
	}

	// Set its MTU
	_, err = exec.Command("ip", "link", "set", "dev", t.Name(), "mtu", strconv.Itoa(tunnel.Mtu)).Output()
	if err != nil {
		return nil, nil, err
	}

	// Bring up the device
	_, err = exec.Command("ip", "link", "set", "dev", t.Name(), "up").Output()
	if err != nil {
		return nil, nil, err
	}

	tunnel.tun = t
	in := tunnel.writeHandler(t)
	out := tunnel.readHandler(t)
	return in, out, err
}

func (tunnel *Tunnel) writeHandler(tun *water.Interface) chan<- *Packet {
	in := make(chan *Packet)
	go func() {
		for {
			p, ok := <-in
			if !ok {
				log.Error("Failed to read from pIn:")
				return
			}

			src_ip := waterutil.IPv4Source(p.Data)
			tunnel.Book.Add(src_ip.String(), p.Sk)
			log.Debug("TUN SENDING: ", src_ip, p)

			_, err := tunnel.tun.Write(p.Data)
			if err != nil {
				log.Error("Error writing to tun!", err)
			}
		}
	}()
	return in
}

func (tunnel *Tunnel) readHandler(tun *water.Interface) <-chan *Packet {
	out := make(chan *Packet, 100)
	go func() {
		for {
			buffer := make([]byte, MTU)
			n, err := tunnel.tun.Read(buffer)
			log.Debug("TUN READ:", n)
			if err != nil {
				log.Error("Error reading from tunnel.")
				return
			}

			dst_ip := waterutil.IPv4Destination(buffer[:n])
			sk, ok := tunnel.Book.getSession(dst_ip.String())
			// Missing session
			if !ok {
				log.Debug("ignoring packet: no session key found for ip:", dst_ip)
				log.Debug(buffer[:n])
				continue
			}

			p := new(Packet)
			p.Sk = sk
			p.Data = buffer[:n]
			log.Debug("TUN RECEIVED: ", p, len(out), cap(out))
			out <- p
			log.Debug("TUN HANDLED : ", p)
		}
	}()
	return out
}

func (tunnel Tunnel) SetRoute() error {
	route, err := exec.Command("ip", "route", "show", "default", "0.0.0.0/0").Output()
	if err != nil {
		log.Error("Cannot get the default routing interface")
		return err
	}
	parts := strings.Split(string(route), " ")
	// default via 192.168.1.1 dev wlp3s0  proto static  metric 600
	tunnel.oldGw = parts[2]
	tunnel.oldIfce = parts[4]

	log.Infof("Default route interface is %v with gateway %v\n", tunnel.oldIfce, tunnel.oldGw)
	_, err = exec.Command("ip", "route", "del", "0/0", "via", tunnel.oldGw, "dev", tunnel.oldIfce).Output()
	if err != nil {
		log.Errorf("failed to remove the default route, with error %v\n", err)
		return err
	}

	_, err = exec.Command("ip", "route", "add", tunnel.oldGw, "via", tunnel.oldGw, "dev", tunnel.oldIfce).Output()
	if err != nil {
		log.Errorf("failed to add the route to gateway back to the route table gateway %v on interface %v, with error %v\n", tunnel.oldGw, tunnel.oldIfce, err)
		return err
	}

	_, err = exec.Command("ip", "route", "add", "0/0", "dev", tunnel.tun.Name()).Output()
	if err != nil {
		log.Errorf("failed to add the route to gateway back to the route table gateway %v on interface %v, with error %v\n", tunnel.oldGw, tunnel.oldIfce, err)
		return err
	}

	//_, err = exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-o", ifce, "-j", "MASQUERADE").Output()
	return nil
}

func (tunnel Tunnel) ResetRoute() error {
	log.Infof("Reset route to original value interface %v with gateway %v\n", tunnel.oldIfce, tunnel.oldGw)
	_, err := exec.Command("ip", "route", "del", tunnel.oldGw, "dev", tunnel.oldIfce).Output()
	if err != nil {
		log.Errorf("failed to remove route to gateway %v on interface %v, with error %v\n", tunnel.oldGw, tunnel.oldIfce, err)
		return err
	}

	_, err = exec.Command("ip", "route", "del", "0/0", "dev", tunnel.tun.Name()).Output()
	if err != nil {
		log.Errorf("failed to remove the default route, with error %v\n", err)
		return err
	}

	_, err = exec.Command("ip", "route", "add", "0/0", "via", tunnel.oldGw, "dev", tunnel.oldIfce).Output()
	if err != nil {
		log.Errorf("failed to add the old route back: gateway %v on interface %v, with error %v\n", tunnel.oldGw, tunnel.oldIfce, err)
		return err
	}

	return nil
}
