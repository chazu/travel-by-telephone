#!/bin/bash

# Test script for Travel by Telephone SIP Server
# This script performs basic connectivity tests

echo "🧪 Travel by Telephone - Server Test Script"
echo "============================================"

# Check if the binary exists
if [ ! -f "./travel-by-telephone" ]; then
    echo "❌ Binary not found. Please run 'go build -o travel-by-telephone .' first"
    exit 1
fi

echo "✅ Binary found"

# Get local IP address
LOCAL_IP=$(ifconfig | grep -Eo 'inet (addr:)?([0-9]*\.){3}[0-9]*' | grep -Eo '([0-9]*\.){3}[0-9]*' | grep -v '127.0.0.1' | head -1)

if [ -z "$LOCAL_IP" ]; then
    LOCAL_IP=$(hostname -I | awk '{print $1}')
fi

echo "🌐 Local IP address: $LOCAL_IP"

# Check if port 5060 is available
if lsof -Pi :5060 -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "❌ Port 5060 is already in use. Please stop any other SIP servers."
    echo "   You can check what's using the port with: lsof -i :5060"
    exit 1
fi

echo "✅ Port 5060 is available"

# Check if ports 10000-10010 are available (sample range)
PORTS_IN_USE=0
for port in {10000..10010}; do
    if lsof -Pi :$port -sUDP:LISTEN -t >/dev/null 2>&1; then
        PORTS_IN_USE=$((PORTS_IN_USE + 1))
    fi
done

if [ $PORTS_IN_USE -gt 5 ]; then
    echo "⚠️  Warning: Many RTP ports (10000-10010) are in use. This might cause issues."
    echo "   Consider stopping other applications using these ports."
else
    echo "✅ RTP ports appear to be available"
fi

echo ""
echo "📋 Configuration Summary"
echo "========================"
echo "SIP Server will listen on: $LOCAL_IP:5060"
echo "RTP Server will use ports: 10000-20000"
echo ""
echo "📱 PAP2 Configuration:"
echo "Proxy: $LOCAL_IP:5060"
echo "User ID: 1001"
echo "Password: password (or leave blank)"
echo ""

# Offer to start the server
echo "🚀 Ready to start the server!"
echo ""
read -p "Start the Travel by Telephone server now? (y/n): " -n 1 -r
echo ""

if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🎵 Starting server..."
    echo "   Press Ctrl+C to stop"
    echo ""
    ./travel-by-telephone
else
    echo "👋 Server not started. Run './travel-by-telephone' when ready."
    echo ""
    echo "💡 Quick start commands:"
    echo "   ./travel-by-telephone                    # Start the server"
    echo "   go build -o travel-by-telephone .        # Rebuild if needed"
    echo ""
    echo "📖 See README.md for detailed PAP2 configuration instructions."
fi
