# custom-container-overlay-network

![network](./images/overlay.drawio.png)


## Prerequisites

- VirtualBox 7.x.x
- Ubuntu 22.04.x ISO


## Set up nat network

1. Navigate to Tools -> Network -> NAT Network -> Create ![network](./images/networks-01.png)
2. Select the created NatNetwork and ensure Enable DHCP option is set ![network](./images/networks-02.png)


## Set up vms

We need two VMs. We are going to call them edge-one and edge-two. Below steps can be repeated to set up both VMs. The below example shows how to set up a VM called onshore. Make sure to use the respective VM name edge-one or edge-two instead.

1. Navigate to Machine -> New, enter the required information and select an image the vm should be created from ![vm](./images/vms-01.png)
2. ![vm](./images/vms-02.png)
3. ![vm](./images/vms-03.png)
4. Finish creation ![vm](./images/vms-04.png)
5. Right click the created vm and navigate to Settings -> Network and select the NatNetwork ![network](./images/networks-03.png)
6. Start the vm and follow installation steps. Choose minimal installation ![vm](./images/vms-05.png)
7. Use a simple user name, password and hostname ![vm](./images/vms-06.png)

Optional ssh port forwarding:

1. Open terminal and switch to root user. Run `apt update && apt install openssh-server -y`
2. Get vm ip via ip a command, then stop vm.
3. Navigate to NatNetwork -> Port Forwarding ![vm](./images/vms-07.png)
4. Add a new forwarding rule. Make sure the name and host port differ for all vms ![vm](./images/vms-08.png)


## Set up docker overlay

0. Install ngrok for remote work `curl -s https://ngrok-agent.s3.amazonaws.com/ngrok.asc | sudo tee /etc/apt/trusted.gpg.d/ngrok.asc >/dev/null && echo "deb https://ngrok-agent.s3.amazonaws.com buster main" | sudo tee /etc/apt/sources.list.d/ngrok.list && sudo apt update && sudo apt install ngrok`, `ngrok config add-authtoken <token>`
1. Set up dockers apt repository:
```
sudo apt-get update
sudo apt-get install ca-certificates curl gnupg
sudo install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg
sudo chmod a+r /etc/apt/keyrings/docker.gpg

echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update

sudo apt-get install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```
3. Configure /etc/docker/daemon.json with `"bip: "172.17.1.1/24"` for one vm and `"bip": "172.17.2.1/24"` for the other:
```
{
"bip": "<the-respective-bip>"
}
```
2. Set up wireguard:
```
# Both vms:
apt install wireguard
umask 077
wg genkey | tee privatekey | wg pubkey > publickey
```
3. Configure wireguard /etc/wireguard/wg0.conf on VM edge-one:
```
[Interface]
Address = 172.17.0.1/24
ListenPort = 51820
PrivateKey = 4KZvPYABegksSHdiKmTaHFnpiXeO5/AQj6L7a8YjSlE=

[Peer]
PublicKey = Df3ptNMWulL5b4POzLyY59N+5tw59X9liDXuWEglA1Y=
AllowedIPs = 172.17.0.2/24, 172.18.2.0/24
Endpoint = 10.0.2.10:51820
PersistentKeepalive = 25
```
4. Configure wireguard /etc/wireguard/wg0.conf on VM edge-two:
```
[Interface]
Address = 172.17.0.2/24
ListenPort = 51820
PrivateKey = yMAlixuGy+KT29g8CLSNyF0sXV1ouwLLiHUCSaeFt1s=

[Peer]
PublicKey = 9ffGDXtrBiJRaZtZXI8m7cg4ERcG+VomDPmoP+97bTs=
AllowedIPs = 172.17.0.1/24, 172.18.1.0/24
Endpoint =  10.0.2.9:51820
PersistentKeepalive = 25
```
5. Up interfaces on both vms using `systemctl enable wg-quick@wg0.service` and `systemctl start wg-quick@wg0.service`
6. Set up venv and deps:
```
apt install python3.10-venv
apt install python3-pip
python3 -m venv venv
source venv/bin/activate
pip install aiodocker
```
7. Create compose.yaml with docker proxy on VM edge-one. This will also create a new docker network. We do this to ensure containers can communicate via container name. Only custom docker networks use dockers inbuild DNS. The default network/bridge does not.
```
services:
  proxy:
    container_name: "proxy-edge-one"
    environment:
      - CONTAINERS=1
    image: "tecnativa/docker-socket-proxy"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - 172.17.0.1:2375:2375
    privileged: true
    networks:
      - overlay
    restart: unless-stopped

networks:
  overlay:
    name: overlay
    driver: bridge
    ipam:
      driver: default
      config:
        - subnet: 172.18.1.0/24
          gateway: 172.18.1.1
    driver_opts:
      com.docker.network.bridge.name: overlay
```
8. Create compose.yaml with docker proxy on VM edge-two. This will also create a new docker network. We do this to ensure containers can communicate via container name. Only custom docker networks use dockers inbuild DNS. The default network/bridge does not.
```
services:
  proxy:
    container_name: "proxy-edge-two"
    environment:
      - CONTAINERS=1
    image: "tecnativa/docker-socket-proxy"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - 172.17.0.2:2375:2375
    privileged: true
    networks:
      - overlay
    restart: unless-stopped

networks:
  overlay:
    name: overlay
    driver: bridge
    ipam:
      driver: default
      config:
        - subnet: 172.18.2.0/24
          gateway: 172.18.2.1
    driver_opts:
      com.docker.network.bridge.name: overlay
```
9. Start containers using `docker compose up -d`
10. Set up dnsmasq:
```
apt install dnsmasq
systemctl start dnsmasq
systemctl enable dnsmasq
```
11. Configure /etc/dnsmasq.conf on VM edge-one:
```
no-resolv
no-poll
server=8.8.8.8
addn-hosts=/etc/dnsmasq.hosts
```
12. Configure /etc/dnsmasq.conf on VM edge-two:
```
no-resolv
no-poll
server=8.8.8.8
addn-hosts=/etc/dnsmasq.hosts
```
13. Restart dnsmasq using `systemctl restart dnsmasq`
14. Configure /etc/systemd/resolved.conf on VM edge-one:
```
[Resolve]
DNSStubListener=no
DNS=10.0.2.9
```
15. Configure /etc/systemd/resolved.conf on VM edge-two:
```
[Resolve]
DNSStubListener=no
DNS=10.0.2.10
```
16. On both VMs `systemctl restart systemd-resolved`
17. Enable ip forwarding using /etc/sysctl.conf:
```
net.ipv4.ip_forward=1
```
We do this for simplicity reasons in an actual production setup we would use an explicit iptable rule:
`iptables -A FORWARD -i wg0 -o docker0 -j ACCEPT`
Note:
  Currently we have to add the ip tables rule again on every reboot.

18. Enable container host sync from edge-one to edge-two:
```
export DOCKER_HOST=tcp://172.17.0.2:2375
source venv/bin/activate
python3 container-host-sync.py
```
19. Enable container host sync from edge-two to edge-one:
```
export DOCKER_HOST=tcp://172.17.0.1:2375
source venv/bin/activate
python3 container-host-sync.py
```
20. Configure network and VLANs using /etc/systemd/network/ files on VM edge-one:
```
# /etc/systemd/network/00-enp0s3.network
[Match]
Name=enp0s3

[Network]
DHCP=none
VLAN=enp0s3.20
Gateway=10.0.2.1

[Address]
Address=10.0.2.9/24
```
```
# /etc/systemd/network/10-enp0s3.20.network
[Match]
Name=enp0s3.20

[Network]
DHCP=no

[Address]
Address=192.168.0.25/24
```
```
# /etc/systemd/network/10-enp0s3.20.netdev
[NetDev]
Name=enp0s3.20
Kind=vlan

[VLAN]
Id=20
```
21. Configure network and VLANs using /etc/systemd/network/ files on VM edge-two:
```
# /etc/systemd/network/00-enp0s3.network
[Match]
Name=enp0s3

[Network]
DHCP=none
VLAN=enp0s3.20
Gateway=10.0.2.1

[Address]
Address=10.0.2.10/24
```
```
# /etc/systemd/network/10-enp0s3.20.network
[Match]
Name=enp0s3.20

[Network]
DHCP=no

[Address]
Address=192.168.0.26/24
```
```
# /etc/systemd/network/10-enp0s3.20.netdev
[NetDev]
Name=enp0s3.20
Kind=vlan

[VLAN]
Id=20
```
22. Enable and start networkd using `systemctl enable systemd-networkd.service` and `systemctl start systemd-networkd.service`