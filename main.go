package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// SIP server configuration
	SIP_PORT = 5060

	// RTP configuration
	RTP_PORT_MIN = 10000
	RTP_PORT_MAX = 20000

	// Audio configuration
	SAMPLE_RATE = 8000
	FRAME_SIZE  = 160 // 20ms at 8kHz

	// Dial tone frequencies (North American standard)
	DIAL_TONE_FREQ1 = 350.0 // Hz
	DIAL_TONE_FREQ2 = 440.0 // Hz
)

// SIPServer represents our SIP server instance
type SIPServer struct {
	conn         *net.UDPConn
	rtpPort      int
	rtpConn      *net.UDPConn
	registeredUA map[string]*RegisteredUA // Track registered user agents
}

// RegisteredUA represents a registered SIP user agent (like our PAP2)
type RegisteredUA struct {
	Contact    string
	Expires    time.Time
	CallID     string
	RemoteAddr *net.UDPAddr
}

// CallSession represents an active call session
type CallSession struct {
	CallID         string
	RemoteAddr     *net.UDPAddr
	RemoteRTPAddr  *net.UDPAddr
	DialToneActive bool
}

func main() {
	// Parse command line flags
	bindIP := flag.String("ip", "", "IP address to bind to (default: auto-detect)")
	help := flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		fmt.Println("Travel by Telephone - SIP Server for PAP2")
		fmt.Println("=========================================")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  ./travel-by-telephone                    # Bind to all interfaces")
		fmt.Println("  ./travel-by-telephone -ip 192.168.1.100 # Bind to specific IP")
		fmt.Println("  ./travel-by-telephone -help             # Show this help")
		fmt.Println()
		fmt.Println("Network Setup:")
		fmt.Println("  If your PAP2 is on a different subnet (e.g., 192.168.1.0)")
		fmt.Println("  and your computer is on WiFi (e.g., 192.168.5.0), you need:")
		fmt.Println("  1. USB-to-Ethernet adapter connected to PAP2's network")
		fmt.Println("  2. Run with -ip flag using the adapter's IP address")
		fmt.Println()
		fmt.Println("See NETWORKING-SOLUTIONS.md for detailed setup instructions.")
		return
	}

	fmt.Println("Starting Travel by Telephone - SIP Server for PAP2")
	fmt.Println("================================================")

	// Show all available network interfaces
	showNetworkInterfaces()

	// Create SIP server
	server, err := NewSIPServer(*bindIP)
	if err != nil {
		log.Fatalf("Failed to create SIP server: %v", err)
	}
	defer server.Close()

	// Start the server
	fmt.Printf("SIP Server listening on port %d\n", SIP_PORT)
	fmt.Printf("RTP Server listening on port %d\n", server.rtpPort)
	fmt.Println("\nWaiting for PAP2 to register...")
	fmt.Println("Configure your PAP2 to use this server's IP address")

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	go server.Run()

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down server...")
}

// NewSIPServer creates a new SIP server instance
func NewSIPServer(bindIP string) (*SIPServer, error) {
	// Determine bind address
	var sipAddrStr string
	if bindIP != "" {
		sipAddrStr = fmt.Sprintf("%s:%d", bindIP, SIP_PORT)
		fmt.Printf("🎯 Binding to specific IP: %s\n", sipAddrStr)
	} else {
		sipAddrStr = fmt.Sprintf(":%d", SIP_PORT)
		fmt.Printf("🌐 Binding to all interfaces on port %d\n", SIP_PORT)
	}

	// Create UDP connection for SIP
	sipAddr, err := net.ResolveUDPAddr("udp", sipAddrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SIP address: %v", err)
	}

	sipConn, err := net.ListenUDP("udp", sipAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on SIP port: %v", err)
	}

	// Find available RTP port
	rtpPort, rtpConn, err := findAvailableRTPPort()
	if err != nil {
		sipConn.Close()
		return nil, fmt.Errorf("failed to find available RTP port: %v", err)
	}

	return &SIPServer{
		conn:         sipConn,
		rtpPort:      rtpPort,
		rtpConn:      rtpConn,
		registeredUA: make(map[string]*RegisteredUA),
	}, nil
}

// findAvailableRTPPort finds an available port in the RTP range
func findAvailableRTPPort() (int, *net.UDPConn, error) {
	for port := RTP_PORT_MIN; port <= RTP_PORT_MAX; port += 2 { // RTP uses even ports
		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}

		conn, err := net.ListenUDP("udp", addr)
		if err != nil {
			continue
		}

		return port, conn, nil
	}

	return 0, nil, fmt.Errorf("no available RTP ports in range %d-%d", RTP_PORT_MIN, RTP_PORT_MAX)
}

// Close closes the server connections
func (s *SIPServer) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.rtpConn != nil {
		s.rtpConn.Close()
	}
}

// Run starts the main server loop
func (s *SIPServer) Run() {
	buffer := make([]byte, 4096)

	fmt.Printf("🎧 SIP Server ready and listening for packets...\n")

	for {
		n, remoteAddr, err := s.conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("❌ Error reading UDP packet: %v", err)
			continue
		}

		// Parse SIP message
		message := string(buffer[:n])
		fmt.Printf("\n📨 Received SIP Message from %s (%d bytes)\n", remoteAddr, n)
		fmt.Printf("--- Message Content ---\n")
		fmt.Print(message)
		fmt.Printf("--- End Message ---\n")

		// Handle the SIP message
		go s.handleSIPMessage(message, remoteAddr)
	}
}

// handleSIPMessage processes incoming SIP messages
func (s *SIPServer) handleSIPMessage(message string, remoteAddr *net.UDPAddr) {
	// Parse the SIP message to determine the method
	lines := splitLines(message)
	if len(lines) == 0 {
		return
	}

	requestLine := lines[0]

	if isRequest(requestLine) {
		method := getMethod(requestLine)
		switch method {
		case "REGISTER":
			s.handleRegister(message, remoteAddr)
		case "INVITE":
			s.handleInvite(message, remoteAddr)
		case "ACK":
			s.handleAck(message, remoteAddr)
		case "BYE":
			s.handleBye(message, remoteAddr)
		case "OPTIONS":
			s.handleOptions(message, remoteAddr)
		default:
			log.Printf("Unhandled SIP method: %s", method)
		}
	} else {
		// This is a response, not a request
		log.Printf("Received SIP response: %s", requestLine)
	}
}

// Helper functions for SIP message parsing
func splitLines(message string) []string {
	lines := []string{}
	current := ""

	for _, char := range message {
		if char == '\r' {
			continue
		}
		if char == '\n' {
			if current != "" {
				lines = append(lines, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		lines = append(lines, current)
	}

	return lines
}

func isRequest(line string) bool {
	return len(line) > 0 && line[0] != 'S' // SIP responses start with "SIP/"
}

func getMethod(requestLine string) string {
	parts := []string{}
	current := ""

	for _, char := range requestLine {
		if char == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// showNetworkInterfaces displays all available network interfaces
func showNetworkInterfaces() {
	fmt.Println("\n🌐 Available Network Interfaces:")
	fmt.Println("=================================")

	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("Error getting interfaces: %v\n", err)
		return
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		fmt.Printf("Interface: %s\n", iface.Name)
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					fmt.Printf("  IPv4: %s\n", ipnet.IP.String())

					// Determine which subnet this IP is on
					if ipnet.IP.String()[:10] == "192.168.1." {
						fmt.Printf("  📍 This interface is on PAP2 subnet (192.168.1.0)!\n")
					} else if ipnet.IP.String()[:10] == "192.168.5." {
						fmt.Printf("  📶 This interface is on WiFi subnet (192.168.5.0)\n")
					}
				}
			}
		}
		fmt.Println()
	}

	fmt.Println("💡 For best results, use an IP address on the same subnet as your PAP2")
	fmt.Println("   If you don't see a 192.168.1.x address, consider:")
	fmt.Println("   1. Adding a USB-to-Ethernet adapter connected to PAP2's hub")
	fmt.Println("   2. Using router configuration to bridge subnets")
	fmt.Println("   3. Moving PAP2 to WiFi network")
	fmt.Println()
}

// handleRegister processes SIP REGISTER requests
func (s *SIPServer) handleRegister(message string, remoteAddr *net.UDPAddr) {
	fmt.Println("📞 Handling REGISTER request")

	// Extract headers
	headers := parseHeaders(message)
	callID := headers["Call-ID"]
	contact := headers["Contact"]

	// Store registration (simplified - no authentication for now)
	s.registeredUA[callID] = &RegisteredUA{
		Contact:    contact,
		Expires:    time.Now().Add(3600 * time.Second), // 1 hour
		CallID:     callID,
		RemoteAddr: remoteAddr,
	}

	fmt.Printf("✅ Registered UA: %s\n", contact)

	// Send 200 OK response
	response := fmt.Sprintf("SIP/2.0 200 OK\r\n"+
		"Via: %s\r\n"+
		"From: %s\r\n"+
		"To: %s;tag=12345\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s\r\n"+
		"Contact: %s\r\n"+
		"Expires: 3600\r\n"+
		"Content-Length: 0\r\n"+
		"\r\n", headers["Via"], headers["From"], headers["To"], callID, headers["CSeq"], contact)

	s.sendResponse(response, remoteAddr)
}

// handleOptions processes SIP OPTIONS requests (keep-alive)
func (s *SIPServer) handleOptions(message string, remoteAddr *net.UDPAddr) {
	fmt.Println("🔄 Handling OPTIONS request")

	headers := parseHeaders(message)

	response := fmt.Sprintf("SIP/2.0 200 OK\r\n"+
		"Via: %s\r\n"+
		"From: %s\r\n"+
		"To: %s;tag=12345\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s\r\n"+
		"Allow: INVITE, ACK, BYE, CANCEL, OPTIONS, REGISTER\r\n"+
		"Content-Length: 0\r\n"+
		"\r\n", headers["Via"], headers["From"], headers["To"], headers["Call-ID"], headers["CSeq"])

	s.sendResponse(response, remoteAddr)
}

// handleInvite processes SIP INVITE requests (incoming calls)
func (s *SIPServer) handleInvite(message string, remoteAddr *net.UDPAddr) {
	fmt.Println("📞 Handling INVITE request - Phone going off-hook!")

	headers := parseHeaders(message)
	callID := headers["Call-ID"]

	// Parse SDP from the INVITE to get remote RTP address
	remoteRTPAddr := parseSDPForRTP(message, remoteAddr.IP)

	// Create SDP response offering audio
	localIP := getLocalIP()
	sdpResponse := fmt.Sprintf("v=0\r\n"+
		"o=- 123456 654321 IN IP4 %s\r\n"+
		"s=Travel by Telephone\r\n"+
		"c=IN IP4 %s\r\n"+
		"t=0 0\r\n"+
		"m=audio %d RTP/AVP 0 101\r\n"+
		"a=rtpmap:0 PCMU/8000\r\n"+
		"a=rtpmap:101 telephone-event/8000\r\n"+
		"a=fmtp:101 0-15\r\n"+
		"a=sendrecv\r\n", localIP, localIP, s.rtpPort)

	// Send 200 OK with SDP
	response := fmt.Sprintf("SIP/2.0 200 OK\r\n"+
		"Via: %s\r\n"+
		"From: %s\r\n"+
		"To: %s;tag=54321\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s\r\n"+
		"Contact: <sip:server@%s:%d>\r\n"+
		"Content-Type: application/sdp\r\n"+
		"Content-Length: %d\r\n"+
		"\r\n%s", headers["Via"], headers["From"], headers["To"], callID, headers["CSeq"],
		localIP, SIP_PORT, len(sdpResponse), sdpResponse)

	s.sendResponse(response, remoteAddr)

	// Start dial tone and DTMF detection
	go s.startCallSession(callID, remoteAddr, remoteRTPAddr)
}

// handleAck processes SIP ACK requests
func (s *SIPServer) handleAck(message string, remoteAddr *net.UDPAddr) {
	fmt.Println("✅ Handling ACK request - Call established!")
}

// handleBye processes SIP BYE requests (call termination)
func (s *SIPServer) handleBye(message string, remoteAddr *net.UDPAddr) {
	fmt.Println("📴 Handling BYE request - Call terminated")

	headers := parseHeaders(message)

	response := fmt.Sprintf("SIP/2.0 200 OK\r\n"+
		"Via: %s\r\n"+
		"From: %s\r\n"+
		"To: %s;tag=54321\r\n"+
		"Call-ID: %s\r\n"+
		"CSeq: %s\r\n"+
		"Content-Length: 0\r\n"+
		"\r\n", headers["Via"], headers["From"], headers["To"], headers["Call-ID"], headers["CSeq"])

	s.sendResponse(response, remoteAddr)
}

// Helper functions for SIP message processing

// parseHeaders extracts headers from a SIP message
func parseHeaders(message string) map[string]string {
	headers := make(map[string]string)
	lines := splitLines(message)

	for _, line := range lines {
		if line == "" {
			break // End of headers
		}

		// Skip request line
		if isRequest(line) || line[:3] == "SIP" {
			continue
		}

		// Parse header
		colonIndex := -1
		for i, char := range line {
			if char == ':' {
				colonIndex = i
				break
			}
		}

		if colonIndex > 0 {
			key := line[:colonIndex]
			value := ""
			if colonIndex+1 < len(line) {
				value = line[colonIndex+1:]
				// Trim leading space
				if len(value) > 0 && value[0] == ' ' {
					value = value[1:]
				}
			}
			headers[key] = value
		}
	}

	return headers
}

// sendResponse sends a SIP response to the remote address
func (s *SIPServer) sendResponse(response string, remoteAddr *net.UDPAddr) {
	_, err := s.conn.WriteToUDP([]byte(response), remoteAddr)
	if err != nil {
		log.Printf("Error sending response: %v", err)
	}

	fmt.Printf("\n--- Sent SIP Response to %s ---\n", remoteAddr)
	fmt.Print(response)
	fmt.Println("--- End Response ---")
}

// getLocalIP gets the local IP address
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// parseSDPForRTP extracts the RTP address and port from SDP content
func parseSDPForRTP(message string, defaultIP net.IP) *net.UDPAddr {
	lines := splitLines(message)
	inSDP := false
	var connectionIP net.IP
	var mediaPort int

	for _, line := range lines {
		if line == "" {
			inSDP = true
			continue
		}

		if !inSDP {
			continue
		}

		// Parse connection information: c=IN IP4 <address>
		if len(line) > 2 && line[:2] == "c=" {
			parts := []string{}
			current := ""
			for _, char := range line {
				if char == ' ' {
					if current != "" {
						parts = append(parts, current)
						current = ""
					}
				} else {
					current += string(char)
				}
			}
			if current != "" {
				parts = append(parts, current)
			}

			if len(parts) >= 3 && parts[1] == "IP4" {
				ip := net.ParseIP(parts[2])
				if ip != nil {
					connectionIP = ip
				}
			}
		}

		// Parse media information: m=audio <port> RTP/AVP ...
		if len(line) > 2 && line[:2] == "m=" {
			parts := []string{}
			current := ""
			for _, char := range line {
				if char == ' ' {
					if current != "" {
						parts = append(parts, current)
						current = ""
					}
				} else {
					current += string(char)
				}
			}
			if current != "" {
				parts = append(parts, current)
			}

			if len(parts) >= 3 && parts[0] == "m=audio" {
				// Parse port number
				port := 0
				for _, char := range parts[1] {
					if char >= '0' && char <= '9' {
						port = port*10 + int(char-'0')
					} else {
						break
					}
				}
				if port > 0 {
					mediaPort = port
				}
			}
		}
	}

	// Use connection IP if found, otherwise use default
	if connectionIP == nil {
		connectionIP = defaultIP
	}

	if mediaPort > 0 {
		return &net.UDPAddr{
			IP:   connectionIP,
			Port: mediaPort,
		}
	}

	return nil
}

// startCallSession starts a call session with dial tone and DTMF detection
func (s *SIPServer) startCallSession(callID string, remoteAddr *net.UDPAddr, remoteRTPAddr *net.UDPAddr) {
	fmt.Printf("🎵 Starting call session for Call-ID: %s\n", callID)

	if remoteRTPAddr != nil {
		fmt.Printf("🎯 Remote RTP address: %s\n", remoteRTPAddr)
	}

	session := &CallSession{
		CallID:         callID,
		RemoteAddr:     remoteAddr,
		RemoteRTPAddr:  remoteRTPAddr,
		DialToneActive: true,
	}

	// Start dial tone generation
	go s.generateDialTone(session)

	// Start DTMF detection
	go s.detectDTMF(session)
}

// generateDialTone generates and streams dial tone audio
func (s *SIPServer) generateDialTone(session *CallSession) {
	fmt.Println("🎵 Starting dial tone generation...")

	// Generate dial tone samples (350Hz + 440Hz)
	samples := make([]int16, FRAME_SIZE)
	sampleIndex := 0

	// RTP packet structure
	rtpHeader := make([]byte, 12)
	rtpHeader[0] = 0x80 // Version 2, no padding, no extension, no CSRC
	rtpHeader[1] = 0x00 // Payload type 0 (PCMU)

	sequenceNumber := uint16(0)
	timestamp := uint32(0)
	ssrc := uint32(0x12345678)

	ticker := time.NewTicker(20 * time.Millisecond) // 20ms frames
	defer ticker.Stop()

	for session.DialToneActive {
		select {
		case <-ticker.C:
			// Generate audio samples for this frame
			for i := 0; i < FRAME_SIZE; i++ {
				t := float64(sampleIndex) / SAMPLE_RATE

				// Generate dual-tone (350Hz + 440Hz)
				sample1 := 0.5 * math.Sin(2*math.Pi*DIAL_TONE_FREQ1*t)
				sample2 := 0.5 * math.Sin(2*math.Pi*DIAL_TONE_FREQ2*t)
				combined := sample1 + sample2

				// Convert to 16-bit PCM
				samples[i] = int16(combined * 16383) // Scale to 14-bit for μ-law
				sampleIndex++
			}

			// Convert to μ-law
			ulawData := make([]byte, FRAME_SIZE)
			for i, sample := range samples {
				ulawData[i] = linearToUlaw(sample)
			}

			// Build RTP packet
			binary.BigEndian.PutUint16(rtpHeader[2:4], sequenceNumber)
			binary.BigEndian.PutUint32(rtpHeader[4:8], timestamp)
			binary.BigEndian.PutUint32(rtpHeader[8:12], ssrc)

			// Combine header and payload
			rtpPacket := append(rtpHeader, ulawData...)

			// Send RTP packet to remote address if available
			if session.RemoteRTPAddr != nil {
				_, err := s.rtpConn.WriteToUDP(rtpPacket, session.RemoteRTPAddr)
				if err != nil {
					log.Printf("Error sending RTP packet: %v", err)
				}
			}

			sequenceNumber++
			timestamp += FRAME_SIZE

		default:
			// Non-blocking check
		}
	}

	fmt.Println("🔇 Dial tone stopped")
}

// detectDTMF listens for DTMF events on the RTP stream
func (s *SIPServer) detectDTMF(session *CallSession) {
	fmt.Println("🎯 Starting DTMF detection...")

	buffer := make([]byte, 1500) // Max UDP packet size

	for {
		// Set read timeout
		s.rtpConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		n, remoteAddr, err := s.rtpConn.ReadFromUDP(buffer)
		if err != nil {
			// Check if it's a timeout
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			log.Printf("Error reading RTP packet: %v", err)
			continue
		}

		if n < 12 {
			continue // Too small to be valid RTP
		}

		// Parse RTP header
		payloadType := buffer[1] & 0x7F

		// Check if this is a DTMF event (payload type 101)
		if payloadType == 101 {
			if n >= 16 { // RTP header (12) + DTMF event (4)
				event := buffer[12]
				//volume := buffer[13]
				//duration := binary.BigEndian.Uint16(buffer[14:16])

				digit := dtmfEventToDigit(event)
				if digit != "" {
					fmt.Printf("🔢 DTMF Detected: %s (from %s)\n", digit, remoteAddr)

					// Stop dial tone on first digit
					if session.DialToneActive {
						session.DialToneActive = false
						fmt.Println("🔇 Stopping dial tone - digit detected")
					}
				}
			}
		}
	}
}

// Audio codec helper functions

// linearToUlaw converts 16-bit linear PCM to μ-law
func linearToUlaw(sample int16) byte {
	// μ-law compression algorithm
	const BIAS = 0x84
	const CLIP = 32635

	var sign, expt, mantissa byte
	var ulawbyte byte

	// Get the sample into sign-magnitude
	if sample < 0 {
		sample = -sample
		sign = 0x80
	} else {
		sign = 0
	}

	// Clip the magnitude
	if sample > CLIP {
		sample = CLIP
	}

	// Convert from 16 bit linear to μ-law
	sample = sample + BIAS
	expt = 7
	for i := int16(0x4000); i != 0; i >>= 1 {
		if sample&i != 0 {
			break
		}
		expt--
	}
	mantissa = byte((sample >> (expt + 3)) & 0x0F)
	ulawbyte = ^(sign | (expt << 4) | mantissa)

	return ulawbyte
}

// dtmfEventToDigit converts DTMF event code to digit string
func dtmfEventToDigit(event byte) string {
	switch event {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	case 3:
		return "3"
	case 4:
		return "4"
	case 5:
		return "5"
	case 6:
		return "6"
	case 7:
		return "7"
	case 8:
		return "8"
	case 9:
		return "9"
	case 10:
		return "*"
	case 11:
		return "#"
	case 12:
		return "A"
	case 13:
		return "B"
	case 14:
		return "C"
	case 15:
		return "D"
	default:
		return ""
	}
}
