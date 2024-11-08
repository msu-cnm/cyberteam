package main

import (
	"crypto/rand"
	"crypto/rsa"
	"github.com/creack/pty"
	"os/exec"
	"fmt"
	"log"
	"net"
	"encoding/binary"
	"golang.org/x/crypto/ssh"
	"os"
	"syscall"
	"unsafe"
	"io"
	"time"
	"errors"
	"math/big"
	"encoding/gob"
	"io/ioutil"
	"encoding/json"
	"tunnelbees/schnorr"
	"tunnelbees/crypto"
	"flag"
	"sync"
)

var (
  errBadPassword = errors.New("permission denied")
  privateKey, _ = rsa.GenerateKey(rand.Reader, 2048)
  signer, _ = ssh.NewSignerFromSigner(privateKey)
  stopChannels = make(map[int]chan struct{})
  switchStopChannels = make(map[int]chan struct{})
  handshakePort *int
  username      *string
  p, g, x, y *big.Int
  stopChannelsMutex sync.Mutex
)

func main() {
  handshakePort = flag.Int("eport", 312, "specified port for ZK handshake")
  key := flag.String("key", "key.json", "Secret key for ZK handshake")
  username = flag.String("username", "testuser", "Username for SSH auth")
  logPath := fmt.Sprintf("/var/log/tunnelbees-%s.log", time.Now().Format("2006-01-02-15-04-05-000"))
  logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)

  flag.Usage = func() {
    fmt.Fprintf(flag.CommandLine.Output(), "Usage of tb-server \n")
    fmt.Println("Sets up a tb-server")
    flag.PrintDefaults()
  }

  if flag.Lookup("help") != nil || flag.Lookup("h") != nil {
    flag.Usage()
    return
  }

  data, err := ioutil.ReadFile(fmt.Sprintf("%s", *key))
  if err != nil {
    fmt.Println("Error reading file:", err)
    return
  }

  // Unmarshal the JSON data into a map
  values := make(map[string]string)
  err = json.Unmarshal(data, &values)
  if err != nil {
    fmt.Println("Error unmarshalling JSON:", err)
    return
  }

  p, _ = new(big.Int).SetString(values["p"], 10)
  g, _ = new(big.Int).SetString(values["g"], 10)
  x, _ = new(big.Int).SetString(values["x"], 10)
  y = new(big.Int).Exp(g, x, p)

  defer logFile.Close()

  go listenToPortMM(*handshakePort)

  wait := make(chan struct{})
  <-wait
}

func listenToPortMM(port int) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		panic(err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Connection error:", err)
			continue
		}
		go handleConnectionMM(conn)
	}
}

func handleConnectionMM(conn net.Conn) {
	defer conn.Close()

  decoder := gob.NewDecoder(conn)

	// Receive public values and commitment from the client
	var clientData struct {
		T *big.Int
	}
	err := decoder.Decode(&clientData)
	if err != nil {
		fmt.Println("Error decoding client data:", err)
    conn.Close()
		return
	}

	// Generate and send challenge to the client
	c := schnorr.VerifierChallenge(p)
	encoder := gob.NewEncoder(conn)
	err = encoder.Encode(c)
	if err != nil {
		fmt.Println("Error encoding challenge:", err)
		return
	}

	// Receive the response from the client
	var s *big.Int
	err = decoder.Decode(&s)
	if err != nil {
		fmt.Println("Error decoding response:", err)
		return
	}

	// Verify the response
	result := schnorr.VerifierCheck(p, g, y, clientData.T, c, s)
	if result {
    pq := new(big.Int)
    pq.SetString("4096", 10)
    port := int(crypto.HashWithSalt(clientData.T, x).Mod(crypto.HashWithSalt(clientData.T, x), pq).Int64())
    if port == 53 || port == *handshakePort { 
      port++
    }

    go switchMM(port, crypto.HashWithSalt(clientData.T,x))

    // kinda jank
		encoder.Encode("vs")

    conn.Close()
	  time.Sleep(10 * time.Second)
    go stopSwitchMM(port)

	} else {
		encoder.Encode("vf")
	}
}

func switchMM(port int, vp *big.Int) {
  stopCh := make(chan struct{})
  switchStopChannels[port] = stopCh

  hostConfig := &ssh.ServerConfig{
	PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
		if c.User() == "gg" {
      passStr := string(pass)
      passBigInt := new(big.Int)
      passBigInt.SetString(passStr, 10)
      if passBigInt.Cmp(vp) == 0 {
          // Password matches
          return nil, nil
      }
    }
		return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

  hostConfig.AddHostKey(signer)

  listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))

  defer func(p int) {
      listener.Close()
  }(port)

  go func() {
      <-stopCh
      listener.Close()
  }()

	if err != nil {
		log.Fatalf("Failed to listen on %s (%v)", port, err)
	}

  for {
      nConn, err := listener.Accept()
      if err != nil {
          if netErr, ok := err.(net.Error); ok && netErr.Temporary() {
              nConn.Close()
              continue
          }
          break
      }
      go handleSSHConnection(nConn, hostConfig)
  }
}

func stopSwitchMM(port int) {
    if ch, ok := switchStopChannels[port]; ok {
        close(ch)
        delete(switchStopChannels, port)
    }
}

func handleSSHConnection(nConn net.Conn, config *ssh.ServerConfig) {
    conn, chans, _, err := ssh.NewServerConn(nConn, config)
    if err != nil {
        log.Printf("Failed to handshake (%v)", err)
        return
    }
    log.Printf("New SSH connection from %s (%s)", conn.RemoteAddr(), conn.ClientVersion())
    defer nConn.Close()

    for newChannel := range chans {
        if newChannel.ChannelType() != "session" {
            newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
            continue
        } else {
          go handleChannel(newChannel)
        }
    }
}

func SetWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func parseDims(b []byte) (width, height int) {
    width = int(binary.BigEndian.Uint32(b))
    height = int(binary.BigEndian.Uint32(b[4:]))
    return
}

func handleChannel(newChannel ssh.NewChannel) {
    channel, requests, err := newChannel.Accept()
    if err != nil {
        log.Printf("Could not accept channel: %v", err)
        return
    }
    defer channel.Close()

    var w, h int
    // Ensure all global requests are serviced.
    go func(in <-chan *ssh.Request) {
        for req := range in {
            switch req.Type {
            case "shell":
                req.Reply(true, nil) 
            case "pty-req":
                // w, h = parseDims(req.Payload[4:]) // Extracting width and height
                req.Reply(true, nil)
            case "window-change":
                // w, h = parseDims(req.Payload)
                // You'd handle terminal resizing here but we need the pty's file descriptor
                continue // no reply for this one
            // We should reply to unknown requests as well, otherwise the SSH client might hang
            default:
                if req.WantReply {
                    req.Reply(false, nil)
                }
            }
        }
    }(requests)

    runShell(channel, w, h)
}

func runShell(ch ssh.Channel, w int, h int) {
  cmd := exec.Command("/bin/sh")
  
  // Start the command with a pty.
  ptmx, err := pty.Start(cmd)
  if err != nil {
      log.Printf("Failed to start command with pty: %v", err)
      return
  }
  defer func() { _ = ptmx.Close() }() // Safely ignore the error from closing

  // SetWinsize(ptmx, w, h)

  go func() {
      _, _ = io.Copy(ch, ptmx)
      ch.Close()
  }()
  go func() {
      _, _ = io.Copy(ptmx, ch)
  }()

  cmd.Wait()
  ch.Close()
}
