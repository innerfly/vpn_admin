#!/bin/bash
#
# OpenVPN Add User Script
# Extracted from https://github.com/Nyr/openvpn-install
#
# This script allows you to create a new OpenVPN user and generate their configuration file
# without running the full installation script.

# Detect Debian users running the script with "sh" instead of bash
if readlink /proc/$$/exe | grep -q "dash"; then
	echo 'This script needs to be run with "bash", not "sh".'
	exit 1
fi

# Discard stdin. Needed when running from a one-liner which includes a newline
read -N 999999 -t 0.001

# Check if OpenVPN is already installed
if [[ ! -e /etc/openvpn/server/server.conf ]]; then
	echo "OpenVPN is not installed. Please run the installation script first."
	exit 1
fi

# Check if running as root
if [[ "$EUID" -ne 0 ]]; then
	echo "This script needs to be run with superuser privileges."
	exit 1
fi

# Detect OS
if grep -qs "ubuntu" /etc/os-release; then
	os="ubuntu"
	group_name="nogroup"
elif [[ -e /etc/debian_version ]]; then
	os="debian"
	group_name="nogroup"
elif [[ -e /etc/almalinux-release || -e /etc/rocky-release || -e /etc/centos-release ]]; then
	os="centos"
	group_name="nobody"
elif [[ -e /etc/fedora-release ]]; then
	os="fedora"
	group_name="nobody"
else
	echo "This script only works on Ubuntu, Debian, AlmaLinux, Rocky Linux, CentOS and Fedora."
	exit 1
fi

# Store the absolute path of the directory where the script is located
script_dir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Get client name
echo
echo "Provide a name for the client:"
read -p "Name: " unsanitized_client
client=$(sed 's/[^0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-]/_/g' <<< "$unsanitized_client")

# Check if client name is valid and not already used
while [[ -z "$client" || -e /etc/openvpn/server/easy-rsa/pki/issued/"$client".crt ]]; do
	echo "$client: invalid name or already exists."
	read -p "Name: " unsanitized_client
	client=$(sed 's/[^0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ_-]/_/g' <<< "$unsanitized_client")
done

# Change to the easy-rsa directory
cd /etc/openvpn/server/easy-rsa/ || {
	echo "Failed to change to /etc/openvpn/server/easy-rsa/ directory."
	echo "Make sure OpenVPN is properly installed."
	exit 1
}

# Create the client certificate
echo "Creating client certificate..."
./easyrsa --batch --days=3650 build-client-full "$client" nopass

# Check if certificate creation was successful
if [[ ! -e /etc/openvpn/server/easy-rsa/pki/issued/"$client".crt ]]; then
	echo "Failed to create client certificate."
	exit 1
fi

# Build the client .ovpn file
echo "Building client configuration file..."
if [[ ! -e /etc/openvpn/server/client-common.txt ]]; then
	echo "client-common.txt not found. Make sure OpenVPN is properly installed."
	exit 1
fi

if [[ ! -e /etc/openvpn/server/easy-rsa/pki/inline/private/"$client".inline ]]; then
	echo "Client inline private key not found. Make sure OpenVPN is properly installed."
	exit 1
fi

# Build the client .ovpn file, stripping comments from easy-rsa in the process
grep -vh '^#' /etc/openvpn/server/client-common.txt /etc/openvpn/server/easy-rsa/pki/inline/private/"$client".inline > "$script_dir"/"$client".ovpn

echo
echo "$client added. Configuration available in: $script_dir/$client.ovpn"