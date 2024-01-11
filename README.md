# custom-container-overlay-network


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
Address = 172.17.0.2/24
ListenPort = 51820
PrivateKey = yMAlixuGy+KT29g8CLSNyF0sXV1ouwLLiHUCSaeFt1s=

[Peer]
PublicKey = 9ffGDXtrBiJRaZtZXI8m7cg4ERcG+VomDPmoP+97bTs=
AllowedIPs = 172.17.0.1/24, 172.17.1.0/24
Endpoint =  10.0.2.9:51820
PersistentKeepalive = 25
```
4. Configure wireguard /etc/wireguard/wg0.conf on VM edge-two:
```
[Interface]
Address = 172.17.0.1/24
ListenPort = 51820
PrivateKey = 4KZvPYABegksSHdiKmTaHFnpiXeO5/AQj6L7a8YjSlE=

[Peer]
PublicKey = Df3ptNMWulL5b4POzLyY59N+5tw59X9liDXuWEglA1Y=
AllowedIPs = 172.17.0.2/24, 172.17.2.0/24
Endpoint = 10.0.2.10:51820
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
7. Create compose.yaml with docker proxy on VM edge-one:
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
    network_mode: bridge
```
8. Create compose.yaml with docker proxy on VM edge-two:
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
    network_mode: bridge
```
9. Start containers using `docker compose up -d`
10. Set up dnsmasq:
```
apt install dnsmasq
systemctl disable systemd-resolved
systemctl stop systemd-resolved
systemctl start dnsmasq
systemctl enable dnsmasq
```
11. Configure /etc/dnsmasq.conf on VM edge-one:
```
strict-order
server=172.17.0.2
server=8.8.8.8
addn-hosts=/etc/dnsmasq.hosts
```
12. Configure /etc/dnsmasq.conf on VM edge-two:
```
strict-order
server=172.17.0.1
server=8.8.8.8
addn-hosts=/etc/dnsmasq.hosts
```
13. Restart dnsmasq using `systemctl restart dnsmasq`
14. Remove /etc/resol.conf sym links using rm `/etc/resolv.conf`
15. Configure /etc/resolv.conf on VM edge-one:
```
nameserver 10.0.2.9
```
16. Configure /etc/resolv.conf on VM edge-two:
```
nameserver 10.0.2.10
```
17. Enable ip forwarding using /etc/sysctl.conf:
```
net.ipv4.ip_forward=1
```
18. Allow ip forwarding from wg0 to docker0 using `iptables -A FORWARD -i wg0 -o docker0 -j ACCEPT`

Note:
  Currently we have to add the ip tables rule again on every reboot.

19. Enable container host sync from edge-one to edge-two:
```
export DOCKER_HOST=tcp://172.17.0.2:2375
source venv/bin/activate
python3 container-host-sync.py
```
20. Enable container host sync from edge-two to edge-one:
```
export DOCKER_HOST=tcp://172.17.0.1:2375
source venv/bin/activate
python3 container-host-sync.py
```
21. Create network config files in /etc/systemd/network
22. Enable and start networkd using `systemctl enable systemd-networkd.service` and `systemctl enable systemd-networkd.service`