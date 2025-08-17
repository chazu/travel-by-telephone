# Travel by Telephone - SIP Server for PAP2

A Go program that acts as a SIP server to interface with your Linksys PAP2 analog phone adapter, providing dial tone and detecting DTMF digits in real-time.

## Features

- 🎵 **Dial Tone Generation**: Provides North American standard dial tone (350Hz + 440Hz)
- 🔢 **DTMF Detection**: Real-time detection and display of pressed digits (0-9, *, #, A-D)
- 📞 **SIP Server**: Full SIP server implementation handling REGISTER, INVITE, OPTIONS, ACK, BYE
- 🎯 **RTP Audio Streaming**: μ-law codec support for telephony-grade audio
- 🔄 **Automatic Registration**: Handles PAP2 registration and keep-alive messages

## Requirements

- Go 1.24.5 or later
- Linksys PAP2 or PAP2T analog phone adapter
- Analog phone connected to the PAP2
- Network connectivity between your computer and PAP2

## Quick Start

1. **Build the program:**
   ```bash
   go build -o travel-by-telephone .
   ```

2. **Run the SIP server:**
   ```bash
   ./travel-by-telephone
   ```

3. **Configure your PAP2** (see detailed instructions below)

4. **Pick up the phone** and start dialing!

## PAP2 Configuration

### Step 1: Access PAP2 Web Interface

1. Find your PAP2's IP address (check your router's DHCP client list)
2. Open a web browser and navigate to `http://[PAP2_IP_ADDRESS]`
3. Click on "Admin Login" and then "advanced"

### Step 2: Configure Line 1

Navigate to the **Line 1** tab and configure the following settings:

#### Basic Settings
- **Proxy**: `[YOUR_COMPUTER_IP]:5060` (replace with your computer's IP address)
- **Register**: `Yes`
- **Make Call Without Reg**: `No`
- **User ID**: `1001` (or any username you prefer)
- **Password**: `password` (or leave blank - authentication is disabled for simplicity)
- **Display Name**: `PAP2 Phone`

#### SIP Settings
- **SIP Port**: `5060`
- **Register Expires**: `3600`

#### Audio Configuration
- **Preferred Codec**: `G711u` (μ-law)
- **Use Pref Codec Only**: `Yes`
- **DTMF Tx Method**: `RFC2833`
- **DTMF Tx Mode**: `Strict`

#### Network Settings
- **NAT Mapping Enable**: `No` (if on same subnet)
- **NAT Keep Alive Enable**: `Yes`

### Step 3: Save and Reboot

1. Click **Submit All Changes**
2. Wait for the PAP2 to reboot (about 30-60 seconds)
3. The PAP2 should register with your SIP server

## Usage

1. **Start the server:**
   ```bash
   ./travel-by-telephone
   ```

2. **Look for registration confirmation:**
   ```
   📞 Handling REGISTER request
   ✅ Registered UA: <sip:1001@[PAP2_IP]:5060>
   ```

3. **Pick up the phone:**
   - You should hear a dial tone
   - The server will show: `📞 Handling INVITE request - Phone going off-hook!`

4. **Dial numbers:**
   - Press any digits on the phone
   - The server will display: `🔢 DTMF Detected: [digit]`
   - Dial tone will stop after the first digit

5. **Hang up the phone:**
   - The server will show: `📴 Handling BYE request - Call terminated`

## Example Output

```
Starting Travel by Telephone - SIP Server for PAP2
================================================
SIP Server listening on port 5060
RTP Server listening on port 10000

Waiting for PAP2 to register...
Configure your PAP2 to use this server's IP address

📞 Handling REGISTER request
✅ Registered UA: <sip:1001@192.168.1.100:5060>

📞 Handling INVITE request - Phone going off-hook!
🎵 Starting call session for Call-ID: 1234567890@192.168.1.100
🎯 Remote RTP address: 192.168.1.100:16384
🎵 Starting dial tone generation...
🎯 Starting DTMF detection...

🔢 DTMF Detected: 5 (from 192.168.1.100:16384)
🔇 Stopping dial tone - digit detected
🔢 DTMF Detected: 5 (from 192.168.1.100:16384)
🔢 DTMF Detected: 5 (from 192.168.1.100:16384)
🔢 DTMF Detected: 1 (from 192.168.1.100:16384)
🔢 DTMF Detected: 2 (from 192.168.1.100:16384)
🔢 DTMF Detected: 1 (from 192.168.1.100:16384)
🔢 DTMF Detected: 2 (from 192.168.1.100:16384)

📴 Handling BYE request - Call terminated
```

## Troubleshooting

### PAP2 Won't Register

1. **Check network connectivity:**
   ```bash
   ping [PAP2_IP_ADDRESS]
   ```

2. **Verify server is running:**
   - Ensure the server shows "SIP Server listening on port 5060"
   - Check that no other SIP server is running on port 5060

3. **Check PAP2 configuration:**
   - Verify the Proxy setting points to your computer's IP
   - Ensure SIP Port is set to 5060
   - Try rebooting the PAP2

### No Dial Tone

1. **Check RTP connectivity:**
   - Ensure firewall allows UDP traffic on ports 10000-20000
   - Verify the server shows "Remote RTP address" when call starts

2. **Audio codec issues:**
   - Ensure PAP2 is configured for G711u (μ-law) codec
   - Check "Use Pref Codec Only" is set to Yes

### DTMF Not Detected

1. **Check DTMF configuration:**
   - Verify "DTMF Tx Method" is set to "RFC2833"
   - Ensure "DTMF Tx Mode" is set to "Strict"

2. **Network issues:**
   - Check for packet loss between PAP2 and server
   - Verify RTP packets are reaching the server

### Firewall Issues

If you're having connectivity issues, ensure these ports are open:

- **TCP/UDP 5060**: SIP signaling
- **UDP 10000-20000**: RTP audio streams

## Technical Details

### Supported Features

- **SIP Methods**: REGISTER, INVITE, ACK, BYE, OPTIONS
- **Audio Codec**: μ-law (PCMU) at 8kHz
- **DTMF**: RFC 2833 out-of-band events
- **Audio Format**: 20ms frames, 160 samples per frame

### Architecture

```
[Analog Phone] ←→ [PAP2] ←→ [Network] ←→ [Go SIP Server]
                    ↑                        ↑
                SIP/RTP                 Dial Tone +
               Protocols               DTMF Detection
```

### Dependencies

- `github.com/jart/gosip` - SIP/RTP library for Go

## License

This project is open source. Feel free to modify and distribute.

## Contributing

Contributions are welcome! Please feel free to submit issues or pull requests.
